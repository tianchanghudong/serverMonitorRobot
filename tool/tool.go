package tool

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/astaxie/beego/cache"
	"github.com/axgle/mahonia"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"servermonitorrobot/log"
	"strconv"
	"strings"
	"time"
)

type ExecCommandFunc func(result string)

//阻塞式的执行外部shell命令的函数,等待执行完毕并返回标准输出
func Exec_shell(cmdName, s string) (string, error) {
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	cmd := exec.Command(cmdName, "-c", s)

	//读取io.Writer类型的cmd.Stdout，再通过bytes.Buffer(缓冲byte类型的缓冲器)将byte类型转化为string类型(out.String():这是bytes类型提供的接口)
	var out bytes.Buffer
	cmd.Stdout = &out

	//Run执行c包含的命令，并阻塞直到完成。  这里stdout被取出，cmd.Wait()无法正确获取stdin,stdout,stderr，则阻塞在那了
	err := cmd.Run()
	var enc mahonia.Decoder
	if runtime.GOOS == "windows" {
		enc = mahonia.NewDecoder("gbk")
	} else {
		enc = mahonia.NewDecoder("utf-8")
	}

	return enc.ConvertString(out.String()), err
}

//阻塞式的执行外部shell命令的函数,标准输出的逐行实时进行处理的
func ExecCommand(cmdName, command string, execCommandFunc ExecCommandFunc) bool {
	cmd := exec.Command(cmdName, "-c", command)

	//StdoutPipe方法返回一个在命令Start后与命令标准输出关联的管道。Wait方法获知命令结束后会关闭这个管道，一般不需要显式的关闭该管道。
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error(err)
		return false
	}
	cmd.Start()

	//创建一个流来读取管道内内容，这里逻辑是通过一行一行的读取的
	reader := bufio.NewReader(stdout)
	var enc mahonia.Decoder
	if runtime.GOOS == "windows" {
		enc = mahonia.NewDecoder("gbk")
	} else {
		enc = mahonia.NewDecoder("utf-8")
	}
	//实时循环读取输出流中的一行内容
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			log.Info("err2 != nil || io.EOF == err2")
			break
		}
		if line == "\r\n" {
			continue
		}
		temp := enc.ConvertString(line)
		execCommandFunc(temp)
	}

	//阻塞直到该命令执行完成，该命令必须是被Start方法开始执行的
	cmd.Wait()
	log.Info("执行完毕！" + command)
	execCommandFunc("执行完毕！")
	return true
}

//发送http请求
func Http(requestType, url, content string) (error, []byte) {
	//创建一个请求
	result := ""
	req, err := http.NewRequest(requestType, url, strings.NewReader(content))
	if err != nil {
		result = "发送http请求异常：" + err.Error()
		log.Error(result)
		return errors.New(result), nil
	}

	client := &http.Client{}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		result = "发送http请求失败：" + err.Error()
		log.Error(result)
		return errors.New(result), nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return nil, body
}

var gobDataFilePath = "gobData" //gob文件夹名字
//读取gob文件
func ReadGobFile(fileName string, data interface{}) {
	var dataFile = path.Join(gobDataFilePath, fileName)
	_, err := os.Stat(dataFile)
	if err == nil {
		content, err := ioutil.ReadFile(dataFile)
		if err != nil {
			log.Error("读取用户数据配置文件失败：" + err.Error())
			return
		}
		buf := bytes.NewBuffer(content)
		dec := gob.NewDecoder(buf)
		dec.Decode(data)
	} else {
		_, existPath := os.Stat(gobDataFilePath)
		if nil != existPath {
			os.MkdirAll(gobDataFilePath, os.ModePerm)
		}
	}
}

//保存gob数据
func SaveGobFile(fileName string, _data interface{}) (result string) {
	//编码并存储
	data, errEncodeUser := cache.GobEncode(_data)
	if nil != errEncodeUser {
		result = "编码用户数据失败：" + errEncodeUser.Error()
		log.Error(result)
		return
	}
	fileObj, err := os.Create(path.Join(gobDataFilePath, fileName))
	if err != nil {
		result = "获取用户文件失败：" + err.Error()
		return
	}
	writer := bufio.NewWriter(fileObj)
	defer writer.Flush()
	writer.Write(data)
	return
}

//判断文件是否存在
func CheckFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

//保留float64的前n位小数
func Decimal(value float64, n int32) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%."+fmt.Sprint(n)+"f", value), 64)
	return value
}

//字节转换
func ChangByteValue(value float64) string {
	list := []string{"B", "KB", "MB", "GB", "TB"}
	for index, temp := range list {
		if value/math.Pow(1024, float64(index+1)) >= 1 && index < len(list)-1 {
			continue
		}

		value /= math.Pow(1024, float64(index))
		value = Decimal(value, 1)

		return fmt.Sprintf("%v%v", value, temp)
	}

	return ""
}

func String2Int64(istr string) int64 {
	i, _ := strconv.ParseInt(istr, 10, 64)
	return i
}

func String2Float64(istr string) float64 {
	i, _ := strconv.ParseFloat(istr, 64)
	return i
}

func NewSSHClient(user, password, host string, port int) (*ssh.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		err          error
	)

	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	clientConfig = &ssh.ClientConfig{
		User:    user,
		Auth:    auth,
		Timeout: 5 * time.Second,
		//需要验证服务端，不做验证返回nil就可以，点击HostKeyCallback看源码就知道了
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)

	if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	return client, nil
}

func GetTimeStringByUTC(utc int64) string {
	t := time.Unix(utc, 0)
	return fmt.Sprintf("%d-%02d-%02d_%02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

//获取0点时间戳
func GetBeginTime(t64 int64) int64 {
	t := time.Unix(t64, 0)
	return t.Unix() - int64(t.Hour()*3600+t.Minute()*60+t.Second())
}

//格式化:把addr=127.0.0.1:27017 -> ip=127.0.0.1	port=27017
func FormatDBAddr(addr string) (ip, port string) {
	strList := strings.Split(addr, ":")
	if len(strList) == 2 {
		ip = strList[0]
		port = strList[1]
	}

	return
}

func GoFunc(f func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("r=", r)
		}
	}()

	f()
}
