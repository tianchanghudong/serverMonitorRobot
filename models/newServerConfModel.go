package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/mgo.v2"
	"servermonitorrobot/db/mongodb"
	"servermonitorrobot/log"
	"servermonitorrobot/tool"
	"sync"
	"time"
)

var serverConfigData *ServerConfig                          //服务器通用配置数据
var newServerConfigDataFileName = "newServerConfigData.gob" //服务器配置数据
var allServerConfigData map[string][]*ServerConfig          //每台物理机上的所有游戏服配置 key:所在物理机名称
var allServerConLock sync.Mutex

var CenterDB *mongodb.DialContext // 中心服数据库的链接

//服务器配置
type ServerConfig struct {
	ServerID      string    `bson:"ServerID"`      //通过ID获取数据
	TcpAddr       string    `bson:"TcpAddr"`       //Tcp开放地址
	MaxConnetNum  int       `bson:"MaxConnetNum"`  //最大连接数量
	DBType        string    `bson:"DBType"`        //使用的数据库类型
	ProtoType     string    `bson:"ProtoType"`     //使用的协议类型
	NetBuffLen    int       `bson:"NetBuffLen"`    //数据发送缓冲
	StartTime     time.Time `bson:"StartTime"`     //开服时间
	DBName        string    `bson:"DBName"`        //数据库名称
	WebAddr       string    `bson:"WebAddr"`       //公开的Web端口
	IP            string    `bson:"IP"`            //服务器IP地址
	DBPort        string    `bson:"DBPort"`        //DBPort
	Platform      int       `bson:"Platform"`      //Platform 0 内部，1 安卓，2 苹果
	InnerIP       string    `bson:"InnerIP"`       //内网地址 192.168.1.250
	DBIP          string    `bson:"DBIP"`          //数据库地址
	CSvrTcpAddr   string    `bson:"CSvrTcpAddr"`   //跨服战服务器内网地址
	ServerName    string    `bson:"ServerName"`    //服务器名称
	GroupID       int       `bson:"GroupID"`       //服务器组ID
	ServerType    int       `bson:"ServerType"`    //服务器类型，为1的则为普通玩家，为2为白名单(后台关掉白名单要用到)
	PushID        string    `bson:"PushID"`        //推送ID
	PushTag       string    `bson:"PushTag"`       //推送tag，玩家按服务器做区分
	DirName       string    `bson:"DirName"`       //服务器文件夹名字（一键更新需要）
	ReplSetDBs    []string  `bson:"ReplSetDBs"`    //mongo副本集
	LogDBAddr     string    `bson:"LogDBAddr"`     //日志库地址（包括端口）
	CenterWebPath string    `bson:"CenterWebPath"` //中心服务器地址
	PushWebPath   string    `bson:"PushWebPath"`   //推送服务器地址
	ConfigFolder  string    `bson:"ConfigFolder"`  //配置目录
	LangType      int       `bson:"-"`             //语言类型

	ServerIDPrefix string `bson:"-"` //服务器ID前缀
	DBNamePrefix   string `bson:"-"` //数据库名称前缀
	DirNamePrefix  string `bson:"-"` //服务器文件夹名称前缀

	ServerIndex           string `bson:"-"` //服务器索引
	InstallServerName     string `bson:"-"` //安装物理机的名称
	IsForce               bool   `bson:"-"` //是否强制安装太这台物理机上(游戏服一般间隔安装在两台物理机上)
	LastInstallServerName string `bson:"-"` //上次安装的物理机名称

	RobotScriptName string `bson:"-"` //创建机器人脚本
}

//服务器列表信息
type ServerInfo struct {
	Id                string    `bson:"id"`                //服务器id
	Title             string    `bson:"title"`             //服务器名字
	Ip                string    `bson:"ip"`                //服务器ip
	Port              string    `bson:"port"`              //服务器端口
	State             int       `bson:"state"`             //服务器状态
	ServerType        int       `bson:"ServerType"`        //服务器类型，为1的则为普通玩家，为2为白名单
	StateTips         string    `bson:"statetips"`         //服务器状态提示
	PushTag           string    `bson:"pushtag"`           //推送tag，玩家按服务器做区分
	CombineServerId   string    `bson:"CombineServerId"`   //合服后服务器id
	CombineServerTime time.Time `bson:"CombineServerTime"` //合服时间，只设置合服后的那条配置，如1，2合服，server01,server02配置不设置，只设置合服的server1_2配置
}

//初始化中心服的链接
func InitCenterDB(centerDBAddr string) error {
	var err error
	//先以主库的方式进行连接
	CenterDB, err = mongodb.DialWithMode(centerDBAddr, 10, int(mgo.PrimaryPreferred))
	if err != nil {
		return errors.New(fmt.Sprintf("链接中心服数据库[centerDBAddr=%v]失败!err=%v", centerDBAddr, err.Error()))
	}

	return nil
}

//初始化所有的服务器配置
func InitAllServerConfigData() error {
	if CenterDB == nil {
		return errors.New("链接中心服数据库失败")
	}

	allServerConLock.Lock()
	defer allServerConLock.Unlock()
	serverConfList := make([]*ServerConfig, 0)
	if err := CenterDB.GetTableDataAll("Center", "ServerConfig", &serverConfList); err != nil {
		return err
	}

	allServerConfigData = make(map[string][]*ServerConfig)

	tempMap := make(map[string]string) //缓存一下 key:innerIP value:所在物理机名称
	for _, tempConf := range serverConfList {
		serverName, ok := tempMap[tempConf.InnerIP]
		if !ok {
			serverInfo := GetServerInfoByInnerIP(tempConf.InnerIP)
			if serverInfo == nil {
				continue
			}

			serverName = serverInfo.ServerName
			tempMap[tempConf.InnerIP] = serverName
		}

		tempList := allServerConfigData[serverName]
		tempList = append(tempList, tempConf)
		allServerConfigData[serverName] = tempList
	}

	//输出日志
	serverConfigLog := ""
	for k, serverConfigs := range allServerConfigData {
		temp := k + "主机包含服务器id："
		for _, serverConfig := range serverConfigs {
			temp += (serverConfig.ServerID + "|")
		}
		serverConfigLog += temp + "\n"
	}
	log.Info(serverConfigLog)
	return nil
}

//获取物理机上的所有服务器配置
func GetAllServerConf(serverName string) []*ServerConfig {
	allServerConLock.Lock()
	defer allServerConLock.Unlock()

	return allServerConfigData[serverName]
}

func (this *ServerConfig) ToString() string {
	var serverType string
	if this.ServerType == 2 {
		serverType = fmt.Sprintf("%v|开启白名单", this.ServerType)
	} else if this.ServerType == 1 {
		serverType = fmt.Sprintf("%v|关闭白名单", this.ServerType)
	} else {
		serverType = fmt.Sprintf("%v|未知", this.ServerType)
	}

	return fmt.Sprintf("服务器ID:%v\n服务器名称:%v\nTCP端口:%v\nweb端口:%v\n外网地址:%v\n内网地址:%v\n最大连接数:%v\n服务器组ID:%v\n服务器类型(白名单):%v\n协议类型:%v\n数据发送缓冲:%v\n开服时间:%v\n平台:%v\n"+
		"数据库类型:%v\n数据库名称:%v\n数据库地址:%v\n日志库地址:%v\n中心服地址:%v\n跨服地址:%v\n推送服地址:%v\n推送ID:%v\n推送tag:%v\n服务器文件夹名称:%v\nmongo副本集:%v\n配置目录:%v",
		this.ServerID, this.ServerName, this.TcpAddr, this.WebAddr, this.IP, this.InnerIP, this.MaxConnetNum, this.GroupID, serverType, this.ProtoType, this.NetBuffLen, this.StartTime.String(), this.Platform,
		this.DBType, this.DBName, this.DBIP, this.LogDBAddr, this.CenterWebPath, this.CSvrTcpAddr, this.PushWebPath, this.PushID, this.PushTag, this.DirName, this.ReplSetDBs, this.ConfigFolder)
}

func (this *ServerConfig) ToConfString() (string, string) {
	//初始化默认值
	var configFolder string
	if this.ConfigFolder == "CN" {
		configFolder = fmt.Sprintf("%v|中文简体", this.ConfigFolder)
	} else if this.ConfigFolder == "TW" {
		configFolder = fmt.Sprintf("%v|中文繁体", this.ConfigFolder)
	} else if this.ConfigFolder == "EN" {
		configFolder = fmt.Sprintf("%v|英文", this.ConfigFolder)
	} else if this.ConfigFolder == "THAI" {
		configFolder = fmt.Sprintf("%v|泰文", this.ConfigFolder)
	} else {
		configFolder = fmt.Sprintf("%v|未知", this.ConfigFolder)
	}
	var langType string
	if this.LangType == 0 {
		langType = fmt.Sprintf("%v|中文简体", this.LangType)
	} else if this.LangType == 1 {
		langType = fmt.Sprintf("%v|中文繁体", this.LangType)
	} else if this.LangType == 2 {
		langType = fmt.Sprintf("%v|英文", this.LangType)
	} else if this.LangType == 3 {
		langType = fmt.Sprintf("%v|泰文", this.LangType)
	} else {
		langType = fmt.Sprintf("%v|未知", this.LangType)
	}
	var serverType string
	if this.ServerType == 0 || this.ServerType == 2 {
		this.ServerType = 2
		serverType = fmt.Sprintf("%v|开启白名单", this.ServerType)
	} else {
		serverType = fmt.Sprintf("%v|关闭白名单", this.ServerType)
	}
	if this.LogDBAddr == "" {
		this.LogDBAddr = "mongodb://root:123456@127.0.0.1:27017"
	}
	if this.DBIP == "" {
		this.DBIP = "root:123456@127.0.0.1:27017"
	}
	if this.DirNamePrefix == "" {
		this.DirNamePrefix = "gamesvr"
	}
	if this.CSvrTcpAddr == "" {
		this.CSvrTcpAddr = "127.0.0.1:7040"
	}
	if this.CenterWebPath == "" {
		this.CenterWebPath = "http://127.0.0.1:7050"
	}
	if this.MaxConnetNum == 0 {
		this.MaxConnetNum = 10000
	}
	if this.DBType == "" {
		this.DBType = "mongodb"
	}
	if this.ProtoType == "" {
		this.ProtoType = "pb"
	}
	if this.NetBuffLen == 0 {
		this.NetBuffLen = 100
	}
	if this.PushWebPath == "" {
		this.PushWebPath = "http://127.0.0.1:8080"
	}
	if this.PushID == "" {
		this.PushID = "2"
	}
	if this.PushTag == "" {
		this.PushTag = "tag1"
	}

	jsonStr := fmt.Sprintf("{\"ServerIDPrefix\":\"%v\",\"DBNamePrefix\":\"%v\",\"DirNamePrefix\":\"%v\",\"DBIP\":\"%v\","+
		"\"CSvrTcpAddr\":\"%v\",\"LogDBAddr\":\"%v\",\"CenterWebPath\":\"%v\",\"PushWebPath\":\"%v\",\"ConfigFolder\":\"%v\",\"LangType\":%v,"+
		"\"ServerType\":%v,\"MaxConnetNum\":%v,\"DBType\":\"%v\",\"ProtoType\":\"%v\",\"NetBuffLen\":%v,\"PushID\":\"%v\",\"PushTag\":\"%v\",\"RobotScriptName\":\"%v\"}",
		this.ServerIDPrefix, this.DBNamePrefix, this.DirNamePrefix, this.DBIP,
		this.CSvrTcpAddr, this.LogDBAddr, this.CenterWebPath, this.PushWebPath, this.ConfigFolder, this.LangType,
		this.ServerType, this.MaxConnetNum, this.DBType, this.ProtoType, this.NetBuffLen, this.PushID, this.PushTag, this.RobotScriptName)

	str := fmt.Sprintf("服务器ID前缀:%v\n数据库名称前缀:%v\n服务器文件夹名称前缀:%v\n数据库地址:%v\n跨服地址:%v\n日志库地址:%v\n中心服地址:%v\n推送服地址:%v\n配置目录:%v\n语言类型:%v\n"+
		"服务器类型(白名单):%v\n最大连接数:%v\n数据库类型:%v\n协议类型:%v\n数据发送缓冲:%v\n推送ID:%v\n推送tag:%v\n创建机器人脚本名称:%v\n",
		this.ServerIDPrefix, this.DBNamePrefix, this.DirNamePrefix, this.DBIP, this.CSvrTcpAddr, this.LogDBAddr, this.CenterWebPath, this.PushWebPath, configFolder, langType,
		serverType, this.MaxConnetNum, this.DBType, this.ProtoType, this.NetBuffLen, this.PushID, this.PushTag, this.RobotScriptName)

	return str, jsonStr
}

func (this *ServerInfo) ToString() string {
	var state string
	if this.State == 0 {
		state = fmt.Sprintf("%v|新服", this.State)
	} else if this.State == 1 {
		state = fmt.Sprintf("%v|热服", this.State)
	} else if this.State == 2 {
		state = fmt.Sprintf("%v|维护", this.State)
	} else if this.State == 3 {
		state = fmt.Sprintf("%v|正常", this.State)
	} else {
		state = fmt.Sprintf("%v|未知", this.State)
	}
	var serverType string
	if this.ServerType == 2 {
		serverType = fmt.Sprintf("%v|开启白名单", this.ServerType)
	} else if this.ServerType == 1 {
		serverType = fmt.Sprintf("%v|关闭白名单", this.ServerType)
	} else {
		serverType = fmt.Sprintf("%v|未知", this.ServerType)
	}

	return fmt.Sprintf("服务器ID:%v\n服务器名称:%v\n外网IP:%v\nTCP端口:%v\n服务器状态:%v\n服务器类型(白名单):%v\n服务器状态提示:%v\n推送tag:%v",
		this.Id, this.Title, this.Ip, this.Port, state, serverType, this.StateTips, this.PushTag)
}

func init() {
	serverConfigData = new(ServerConfig)
	tool.ReadGobFile(newServerConfigDataFileName, &serverConfigData)
}

//获取新服的配置数据
func GetNewServerConf() *ServerConfig {
	return serverConfigData
}

//更新新服的配置数据
func SetNewServerConf(lastInstallServerName string) {
	serverConfigData.LastInstallServerName = lastInstallServerName

	//编码并存储
	tool.SaveGobFile(newServerConfigDataFileName, serverConfigData)
}

//获取新服的配置数据
func GetNewServerConfStr() string {
	str, jsonStr := serverConfigData.ToConfString()

	result := "\n***********************以下是已有的新服配置***********************\n" + str
	result += "\n***********************新服配置的json格式***********************\n" + jsonStr
	return result
}

//更新新服的配置数据
func UpdateNewServerConf(confStr string) string {
	if err := json.Unmarshal([]byte(confStr), serverConfigData); err != nil {
		return "解析json字符串失败!err=" + err.Error()
	}

	//编码并存储
	if result := tool.SaveGobFile(newServerConfigDataFileName, serverConfigData); result != "" {
		return result
	}

	return "更新新服配置成功"
}
