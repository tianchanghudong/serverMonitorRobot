package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"servermonitorrobot/db/mongodb"
	"servermonitorrobot/log"
	"servermonitorrobot/models"
	"strconv"
	"strings"
	"time"
)

//指令:更新新服配置数据
func commFuncUpdateNewServerConfData(command models.RobotCommand) string {
	if command.CommandParams == "" {
		//如果为空则列出已配置的数据
		return models.GetNewServerConfStr()
	}

	return models.UpdateNewServerConf(command.CommandParams)
}

//指令:添加新服
func commFuncAddNewServer(command models.RobotCommand) string {
	if command.CommandParams == "" {
		//如果为空则列出json格式字符串
		result := "需要配置以下额外字段 {\"IsForce\":false,\"InstallServerName\":\"安装物理机的名称\",\"ServerIndex\":\"1\",\"ServerName\":\"游戏服1\",\"TcpAddr\":\":7070\",\"WebAddr\":\":7010\",\"Platform\":64,\"GroupID\":1}"

		result += "\n***********************************************\n"
		if newServerConf := models.GetNewServerConf(); newServerConf.LastInstallServerName == "" {
			result += "暂未安装游戏服"
		} else {
			result += fmt.Sprintf("上次安装游戏服的物理机名称:%v", newServerConf.LastInstallServerName)
		}
		return result
	}

	var result string
	//检查添加新服的参数
	newServerExtraConf, err := checkNewServerExtraParam(command.CommandParams)
	if err != nil {
		return "添加新服额外参数错误!err=" + err.Error()
	}
	newServerConf, err2 := checkNewServerParam()
	if err2 != nil {
		return "添加新服参数错误!err=" + err2.Error()
	}

	//判断安装服务器所在的物理机
	if !newServerExtraConf.IsForce && newServerExtraConf.InstallServerName == newServerConf.LastInstallServerName {
		return "和上次安装的物理机相同!建议游戏服间隔安装!"
	}

	//更新上次安装的物理机
	models.SetNewServerConf(newServerExtraConf.InstallServerName)

	//添加服务器配置ServerConfig,ServerInfo到CenterDB
	serverConf, serverInfo, addErr := addServerConfToDB(newServerConf, newServerExtraConf)
	if addErr != nil {
		return addErr.Error()
	}
	result += "创建ServerConfig,ServerInfo插入到CenterDB成功\n"
	result += "ServerConfig配置如下:\n" + serverConf.ToString() + "\n"
	result += "ServerInfo配置如下:\n" + serverInfo.ToString() + "\n"

	//添加机器人
	result += "********************开始添加机器人********************\n"
	result += addRobotToGameSvr(serverConf)
	result += "\n********************添加机器人完毕********************\n"

	result += "\n********************开始给数据库增加索引********************\n"
	result += addIndexToDB(serverConf)
	result += "\n********************给数据库增加索引结束********************\n"

	result += "\n********************开始创建启动/关闭服务器脚本********************\n"
	result += createStartStopServerScript(serverConf)
	result += "\n********************创建启动/关闭服务器脚本结束********************\n"
	return result

	/*result := ""
	serverConf := &models.ServerConfig{
		DBIP:            "mongodb://root:wuyufeng@172.17.0.5:27017",
		DirName:         "gamesvr50002",
		RobotScriptName: "create_robot",
		LogDBAddr:       "mongodb://root:wuyufeng@172.17.0.5:27017",
		DBName:          "C002",
		ConfigFolder:    "CN",
		InnerIP:         "172.17.0.5",
	}

	result += "********************开始测试********************\n"
	result += createStartStopServerScript(serverConf)
	result += "\n********************结束测试*************\n"

	return result*/
}

//检查添加新服的参数
func checkNewServerExtraParam(jsonStr string) (*models.ServerConfig, error) {
	serverConf := new(models.ServerConfig)
	if err := json.Unmarshal([]byte(jsonStr), serverConf); err != nil {
		return nil, err
	}

	//判空
	if serverConf.InstallServerName == "" || serverConf.ServerIndex == "" || serverConf.ServerName == "" || serverConf.TcpAddr == "" || serverConf.WebAddr == "" || serverConf.Platform == 0 || serverConf.GroupID == 0 {
		return nil, errors.New("参数不能为空")
	}

	//获取物理机配置数据
	installServerInfo := models.GetServerInfoByServerName(serverConf.InstallServerName)
	if installServerInfo == nil {
		return nil, errors.New(fmt.Sprintf("安装物理机名称:%v未配置!", serverConf.InstallServerName))
	}

	if _, err := strconv.Atoi(serverConf.ServerIndex); err != nil {
		return nil, errors.New(fmt.Sprint("ServerIndex格式错误!err=", err.Error()))
	}

	//有没有遗漏冒号
	if !strings.Contains(serverConf.TcpAddr, ":") {
		serverConf.TcpAddr = ":" + serverConf.TcpAddr
	}
	if !strings.Contains(serverConf.WebAddr, ":") {
		serverConf.WebAddr = ":" + serverConf.WebAddr
	}

	//赋值外网和内网IP
	serverConf.IP = installServerInfo.IP
	serverConf.InnerIP = installServerInfo.InnerIP

	return serverConf, nil
}

//检查新服配置参数
func checkNewServerParam() (*models.ServerConfig, error) {
	newServerConf := models.GetNewServerConf()

	//判空
	if newServerConf.ServerIDPrefix == "" || newServerConf.DBNamePrefix == "" || newServerConf.DirNamePrefix == "" ||
		newServerConf.CSvrTcpAddr == "" || newServerConf.LogDBAddr == "" || newServerConf.CenterWebPath == "" || newServerConf.PushWebPath == "" ||
		newServerConf.ConfigFolder == "" || newServerConf.ServerType == 0 || newServerConf.MaxConnetNum == 0 || newServerConf.DBType == "" ||
		newServerConf.ProtoType == "" || newServerConf.NetBuffLen == 0 || newServerConf.PushID == "" || newServerConf.PushTag == "" || newServerConf.RobotScriptName == "" {
		return nil, errors.New("参数不能为空,请检查")
	}

	//检查配置目录和语言类型是否一致
	if (newServerConf.ConfigFolder == "CN" && newServerConf.LangType != 0) ||
		(newServerConf.ConfigFolder == "TW" && newServerConf.LangType != 1) ||
		(newServerConf.ConfigFolder == "EN" && newServerConf.LangType != 2) ||
		(newServerConf.ConfigFolder == "THAI" && newServerConf.LangType != 3) {
		return nil, errors.New("配置目录和语言类型不一致,请检查")
	}

	return newServerConf, nil
}

//添加服务器配置ServerConfig,ServerInfo到CenterDB
func addServerConfToDB(newServerConf, newServerExtraConf *models.ServerConfig) (*models.ServerConfig, *models.ServerInfo, error) {
	if models.CenterDB == nil {
		return nil, nil, errors.New(fmt.Sprint("链接中心服数据库失败"))
	}

	//补零
	serverID := newServerConf.ServerIDPrefix
	zeroNum := 4 - len(newServerExtraConf.ServerIndex)
	for i := 0; i < zeroNum; i++ {
		serverID += "0"
	}
	serverID += newServerExtraConf.ServerIndex

	//检查配置是否存在
	if err := models.CenterDB.GetData("Center", "ServerConfig", "ServerID", serverID, new(models.ServerConfig)); err == nil {
		return nil, nil, errors.New(fmt.Sprintf("ServerConfig表中以存在该服务器配置,ServerID=%v", serverID))
	}
	if err := models.CenterDB.GetData("Center", "ServerList", "id", serverID, new(models.ServerInfo)); err == nil {
		return nil, nil, errors.New(fmt.Sprintf("ServerList表中以存在该服务器配置,ServerID=%v", serverID))
	}

	dbName := newServerConf.DBNamePrefix
	zeroNum = 3 - len(newServerExtraConf.ServerIndex)
	for i := 0; i < zeroNum; i++ {
		dbName += "0"
	}
	dbName += newServerExtraConf.ServerIndex

	dirName := "gamesvr" + serverID

	serverConf := &models.ServerConfig{
		ServerID:        serverID,
		TcpAddr:         newServerExtraConf.TcpAddr,
		MaxConnetNum:    newServerConf.MaxConnetNum,
		DBType:          newServerConf.DBType,
		ProtoType:       newServerConf.ProtoType,
		NetBuffLen:      newServerConf.NetBuffLen,
		StartTime:       time.Now().AddDate(1, 0, 0),
		DBName:          dbName,
		WebAddr:         newServerExtraConf.WebAddr,
		IP:              newServerExtraConf.IP,
		Platform:        newServerExtraConf.Platform,
		InnerIP:         newServerExtraConf.InnerIP,
		DBIP:            newServerConf.DBIP,
		CSvrTcpAddr:     newServerConf.CSvrTcpAddr,
		ServerName:      newServerExtraConf.ServerName,
		GroupID:         newServerExtraConf.GroupID,
		ServerType:      5, //5表示新服，这里写死
		PushID:          newServerConf.PushID,
		PushTag:         newServerConf.PushTag,
		DirName:         dirName,
		ReplSetDBs:      []string{},
		LogDBAddr:       newServerConf.LogDBAddr,
		CenterWebPath:   newServerConf.CenterWebPath,
		PushWebPath:     newServerConf.PushWebPath,
		ConfigFolder:    newServerConf.ConfigFolder,
		RobotScriptName: newServerConf.RobotScriptName,
	}
	if err := models.CenterDB.InsertData("Center", "ServerConfig", serverConf); err != nil {
		return nil, nil, errors.New(fmt.Sprint("保存ServerConfig配置失败!err=", err.Error()))
	}

	serverInfo := &models.ServerInfo{
		Id:         serverID,
		Title:      newServerExtraConf.ServerName,
		Ip:         newServerExtraConf.IP,
		Port:       strings.Replace(newServerExtraConf.TcpAddr, ":", "", -1), //这里不需要冒号
		ServerType: 2,                                                        //2表示白名单，这里写死
		PushTag:    newServerConf.PushTag,
	}
	if err := models.CenterDB.InsertData("Center", "ServerList", serverInfo); err != nil {
		return nil, nil, errors.New(fmt.Sprint("保存ServerList配置失败!err=", err.Error()))
	}

	return serverConf, serverInfo, nil
}

//给游戏服新增机器人
func addRobotToGameSvr(serverConf *models.ServerConfig) string {
	commandTxt := fmt.Sprintf("cd toolRelease;chmod +x ./%v;./%v \"mongodb://%v\" \"%v\" %v %v %v", serverConf.RobotScriptName, serverConf.RobotScriptName,
		serverConf.DBIP, serverConf.LogDBAddr, serverConf.DBName, serverConf.ConfigFolder, serverConf.LangType)
	log.Info("增加机器人指令:" + commandTxt)
	return shellCommand(commandTxt, func(msg, executorPhoneNum string) {
		log.Info("msg = ", msg)
	})
}

//给数据增加索引
func addIndexToDB(serverConf *models.ServerConfig) string {
	serverDB, err1 := mongodb.Dial(serverConf.DBIP, 2)
	if err1 != nil {
		return fmt.Sprint("连接游戏服数据库失败!err1=", err1.Error())
	}
	defer func() { serverDB.Close() }()
	logDB, err2 := mongodb.Dial(serverConf.LogDBAddr, 2)
	if err2 != nil {
		return fmt.Sprint("连接游戏服日志库失败!err=", err2.Error())
	}
	defer func() { logDB.Close() }()

	result := ""
	if err := logDB.EnsureIndex(serverConf.DBName, "CreateLog", []string{"CreateTime"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=CreateLog 索引:LoginTime 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := logDB.EnsureIndex(serverConf.DBName, "LoginLog", []string{"LoginTime"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=LoginLog 索引:LoginTime 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := logDB.EnsureIndex(serverConf.DBName, "Production", []string{"Account", "System", "Time"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=Production 索引:Account,System,Time 失败!err=%v\n", serverConf.DBName, err.Error())
	}

	if err := serverDB.EnsureIndex(serverConf.DBName, "PlayerInfo", []string{"Account"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=PlayerInfo 索引:Account 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "PlayerInfo", []string{"PackageName"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=PlayerInfo 索引:PackageName 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "PlayerInfo", []string{"DeviceId"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=PlayerInfo 索引:DeviceId 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "PlayerInfo", []string{"CreateTime"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=PlayerInfo 索引:CreateTime 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "JiangLiInfo", []string{"JiangLiID"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=JiangLiInfo 索引:JiangLiID 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "RechargeList", []string{"AddTime"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=RechargeList 索引:AddTime 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "RechargeList", []string{"PayType"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=RechargeList 索引:PayType 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "RechargeList", []string{"Account"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=RechargeList 索引:Account 失败!err=%v\n", serverConf.DBName, err.Error())
	}
	if err := serverDB.EnsureIndex(serverConf.DBName, "RechargeList", []string{"OrderID"}); err != nil {
		result += fmt.Sprintf("创建DBName=%v TableName=RechargeList 索引:OrderID 失败!err=%v\n", serverConf.DBName, err.Error())
	}

	if result == "" {
		result = "给数据库增加索引成功"
	}

	return result
}

//创建启动/关闭 服务器脚本
func createStartStopServerScript(serverConf *models.ServerConfig) string {
	installServerInfo := models.GetServerInfoByInnerIP(serverConf.InnerIP)
	sshSession, err := models.NewOnceSSHSession(installServerInfo, 22)
	if err != nil {
		return "获取物理机的SSH失败!err=" + err.Error()
	}
	defer func() { _ = sshSession.MyClose() }()

	startScript := fmt.Sprintf("#!/bin/bash\nchmod +x SLGServer\n./SLGServer \\$1 \\\"%v\\\" > output_\\$1 &", serverConf.DBIP)
	stopScript := fmt.Sprintf("#!/bin/bash\npid=\\`ps -elf|grep \\\"SLGServer \\$1\\\"|grep -v \\\"grep\\\"|awk '{print \\$4}'\\`\nif [ -z \\\"\\$pid\\\" ]; then\necho '\"no process to stop\"'\nexit 0 \nfi\n\\`kill -TERM \\$pid\\`")
	command := fmt.Sprintf("cd /data/;mkdir gamesvr;cd gamesvr/;echo -e \"%v\" > start.sh;chmod +x start.sh;echo -e \"%v\" > stop.sh;chmod +x stop.sh;", startScript, stopScript)

	var runResult string
	if runErr := sshSession.Run(command); runErr != nil {
		runResult += "创建start.sh/stop.sh出错!err=" + runErr.Error()
	} else {
		runResult += "创建start.sh/stop.sh成功!"
	}
	runResult += "\n"

	return runResult
}
