package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"servermonitorrobot/tool"
	"strings"
	"sync"
)

const (
	ServerSSHClientErrMax  = 3 //client连接错误最大次数
	ServerSSHSessionErrMax = 3 //ssh连接错误最大次数
)

var serverData map[string]*ServerInfoModel //物理机数据 key:serverName
var serverDataFileName = "serverData.gob"  //用户数据文件
var serverDataLock sync.Mutex

var serverSSHClient map[string]*ssh.Client //各个物理机ssh的连接
var serverSSHClientLock sync.Mutex

type ServerInfoModel struct {
	ServerName string
	IP         string
	InnerIP    string
	UserName   string
	Password   string
	DBPath     string
	DBAddr     string //ip:port
	DBAccount  string //DB账号
	DBPsd      string //DB密码
	IsMaster   bool   //是否为主库
}

func (this *ServerInfoModel) toString() string {
	return fmt.Sprintf("物理机名称:%s,外网地址:%s,内网地址:%s,用户名:%s,密码:%s,DB路径:%s,DB地址:%s,DB账号:%s,DB密码:%s,是否主库:%v",
		this.ServerName, this.IP, this.InnerIP, this.UserName, this.Password, this.DBPath, this.DBAddr, this.DBAccount, this.DBPsd, this.IsMaster)
}

func (this *ServerInfoModel) toJsonString() string {
	return fmt.Sprintf("{\"ServerName\":\"%v\",\"IP\":\"%v\",\"InnerIP\":\"%v\",\"UserName\":\"%v\","+
		"\"Password\":\"%v\",\"DBPath\":\"%v\",\"DBAddr\":\"%v\",\"DBAccount\":\"%v\",\"DBPsd\":\"%v\",\"IsMaster\":%v}",
		this.ServerName, this.IP, this.InnerIP, this.UserName, this.Password, this.DBPath, this.DBAddr, this.DBAccount, this.DBPsd, this.IsMaster)
}

//判断这台物理机上的目录是否存在
func (this *ServerInfoModel) CheckPathIsExists(path string) (bool, error) {
	session, err := NewOnceSSHSession(this, 22)
	if err != nil {
		return false, errors.New(fmt.Sprintf("获取物理机[%v|%v]session失败!err=%v", this.ServerName, this.InnerIP, err.Error()))
	}
	defer func() {
		_ = session.MyClose()
	}()

	commandTxt := fmt.Sprintf("[ -d %v ] && echo yes || echo no", path)
	if err2 := session.Run(commandTxt); err2 != nil {
		return false, errors.New(fmt.Sprintf("执行ssh命令[comm=%v]失败!err=%v", commandTxt, err2.Error()))
	}

	result := session.OutToString()
	if strings.Contains(result, "no") {
		return false, nil
	}

	return true, nil
}

func init() {
	serverData = make(map[string]*ServerInfoModel)
	serverSSHClient = make(map[string]*ssh.Client)
	tool.ReadGobFile(serverDataFileName, &serverData)
}

//获取所有物理机数据
func GetAllServerInfoStr() string {
	serverDataLock.Lock()
	defer serverDataLock.Unlock()

	if len(serverData) <= 0 {
		return "当前没有配置任何物理机,请添加!"
	}

	result := "\n***********************以下是已有的物理机配置***********************\n"
	for _, tempServer := range serverData {
		result += tempServer.toString() + "\n"
	}
	result += "\n***********************以下是已有的物理机配置json格式*****************\n"
	for _, tempServer := range serverData {
		result += tempServer.toJsonString() + "\n"
	}

	return result
}

//获取所有物理机数据
func GetAllServerInfo() []*ServerInfoModel {
	list := make([]*ServerInfoModel, 0)

	serverDataLock.Lock()
	defer serverDataLock.Unlock()

	for _, item := range serverData {
		list = append(list, &ServerInfoModel{ServerName: item.ServerName, IP: item.IP, InnerIP: item.InnerIP, UserName: item.UserName, Password: item.Password,
			DBPath: item.DBPath, DBAddr: item.DBAddr, DBAccount: item.DBAccount, DBPsd: item.DBPsd, IsMaster: item.IsMaster})
	}

	return list
}

//更新物理机信息
func UpdateServerInfo(serverStr string) string {
	serverDataLock.Lock()
	defer serverDataLock.Unlock()

	serverInfoModel := new(ServerInfoModel)
	if err := json.Unmarshal([]byte(serverStr), serverInfoModel); err != nil {
		return "解析json字符串失败!err=" + err.Error()
	}

	//用户名和密码同时为空表示删除该物理机信息
	if serverInfoModel.UserName == "" && serverInfoModel.Password == "" {
		delete(serverData, serverInfoModel.ServerName)
	} else {
		serverData[serverInfoModel.ServerName] = serverInfoModel
	}

	//编码并存储
	if result := tool.SaveGobFile(serverDataFileName, serverData); result != "" {
		return result
	}

	return "更新物理机信息成功"
}

//获取物理机信息
func GetServerInfoByServerName(serverName string) *ServerInfoModel {
	serverDataLock.Lock()
	defer serverDataLock.Unlock()

	return serverData[serverName]
}

//获取物理机信息
func GetServerInfoByInnerIP(innerIP string) *ServerInfoModel {
	serverDataLock.Lock()
	defer serverDataLock.Unlock()

	for _, item := range serverData {
		if item.InnerIP == innerIP {
			return item
		}
	}

	return nil
}

type ServerMemoryInfo struct {
	Total       string  //总内存(带单位)
	Available   string  //可用内存(带单位)
	Used        string  //已用内存(带单位)
	UsedPercent float64 //已用百分比
}

type ServerDiskInfo struct {
	Filesystem  string
	Path        string  //路径
	Total       string  //总内存(带单位)
	Available   string  //可用内存(带单位)
	Used        string  //已用内存(带单位)
	UsedPercent float64 //已用百分比
}

func (info *ServerMemoryInfo) ToString() string {
	return fmt.Sprintf("总内存:%v,可用内存:%v,已用内存:%v,已用百分比:%.1f%%",
		info.Total, info.Available, info.Used, info.UsedPercent)
}

func (info *ServerDiskInfo) ToString() string {
	return fmt.Sprintf("路径:%v,总内存:%v,可用内存:%v,已用内存:%v,已用百分比:%.1f%%",
		info.Path, info.Total, info.Available, info.Used, info.UsedPercent)
}

type SSHSession struct {
	*ssh.Session
	out    bytes.Buffer
	client *ssh.Client //session由该client创建出来的.关闭session的时候连同client一起关闭
}

func (session *SSHSession) OutToString() string {
	return session.out.String()
}

func (session *SSHSession) MyClose() error {
	_ = session.Close()
	_ = session.client.Close()
	return nil
}

/*func NewSSHSession(serverInfo *ServerInfoModel, port int) (*SSHSession, error) {
	if serverInfo == nil {
		return nil, errors.New("serverInfoModel is nil")
	}

	client, err := getSSHClient(serverInfo, port, false)
	if err != nil {
		return nil, err
	}

	var session *ssh.Session
	//最多尝试3次
	for i := 0; i < ServerSSHSessionErrMax; i++ {
		log.Info(fmt.Sprintf("第%d次尝试获取session----", i+1))
		tempSession, tempSessionErr := client.NewSession()
		if tempSessionErr == nil {
			session = tempSession
			break
		} else {
			log.Error("获取session失败err=", tempSessionErr.Error())
		}
	}

	var sessionErr error
	//client获取session失败.client可能过期了?刷新client
	if session == nil {
		newClient, err := getSSHClient(serverInfo, port, true)
		if err != nil {
			return nil, err
		}

		//再最多尝试3次
		for i := 0; i < ServerSSHSessionErrMax; i++ {
			log.Info(fmt.Sprintf("刷新缓存的client后,第%d次尝试获取session----", i+1))
			tempSession, tempSessionErr := newClient.NewSession()
			if tempSessionErr == nil {
				session = tempSession
				break
			}
			sessionErr = tempSessionErr
		}
	}

	//还是获取失败??
	if session == nil {
		return nil, sessionErr
	}

	sshSession := &SSHSession{Session: session}
	sshSession.Stdout = &sshSession.out

	return sshSession, nil
}
*/

/*func getSSHClient(serverInfo *ServerInfoModel, port int, isForce bool) (*ssh.Client, error) {
	serverSSHClientLock.Lock()
	defer serverSSHClientLock.Unlock()

	//从缓存中取
	if !isForce {
		if client := serverSSHClient[serverInfo.ServerName]; client != nil {
			return client, nil
		}
	}

	var newClient *ssh.Client
	var err error
	for i := 0; i < ServerSSHClientErrMax; i++ {
		tempNewClient, tempErr := tool.NewSSHClient(serverInfo.UserName, serverInfo.Password, serverInfo.InnerIP, port)
		if tempErr == nil {
			newClient = tempNewClient
			break
		}
		err = tempErr
	}

	//获取client失败?
	if newClient == nil {
		return nil, err
	}

	serverSSHClient[serverInfo.ServerName] = newClient
	return newClient, nil
}
*/

func NewOnceSSHSession(serverInfo *ServerInfoModel, port int) (*SSHSession, error) {
	if serverInfo == nil {
		return nil, errors.New("serverInfoModel is nil")
	}

	client, err := getOnceSSHClient(serverInfo, port)
	if err != nil {
		return nil, err
	}

	var session *ssh.Session
	//最多尝试3次
	for i := 0; i < ServerSSHSessionErrMax; i++ {
		tempSession, tempSessionErr := client.NewSession()
		if tempSessionErr == nil {
			session = tempSession
			break
		}
		err = tempSessionErr
	}

	if session == nil {
		return nil, err
	}

	sshSession := &SSHSession{Session: session, client: client}
	sshSession.Stdout = &sshSession.out

	return sshSession, nil
}

func getOnceSSHClient(serverInfo *ServerInfoModel, port int) (*ssh.Client, error) {
	var newClient *ssh.Client
	var err error
	for i := 0; i < ServerSSHClientErrMax; i++ {
		tempNewClient, tempErr := tool.NewSSHClient(serverInfo.UserName, serverInfo.Password, serverInfo.InnerIP, port)
		if tempErr == nil {
			newClient = tempNewClient
			break
		}
		err = tempErr
	}

	//获取client失败?
	if newClient == nil {
		return nil, err
	}

	return newClient, nil
}
