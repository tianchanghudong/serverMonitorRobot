//1、所有服务器监控（包括数据库）（是否有严重日志，运行情况等）、主机监控（内存、cpu、网络等），如果有异常情况，主动发给监控群
//2、钉钉群主动请求物理机信息、主机信息、日志信息以及重启服务器
//3、数据库定时全量.增量备份
package manager

import (
	"errors"
	"fmt"
	"servermonitorrobot/log"
	"servermonitorrobot/models"
	"servermonitorrobot/tool"
	"strings"
	"sync"
	"time"
)

//是否检查游戏服正常运行;维护的时候要置为false
var isCheckServerRun = true

//定时检测物理机的信息,如超出设定阈值,主动发送给监控群
func TimerCheckServerInfo() {
	serverInfoList := models.GetAllServerInfo()
	for _, item := range serverInfoList {
		go func(i *models.ServerInfoModel) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("r=", r)
				}
			}()

			checkServerInfo(i)
		}(item)
	}

	time.AfterFunc(time.Duration(ServerTimerCheckInterval)*time.Second, TimerCheckServerInfo)
}

//检查物理机信息
func checkServerInfo(serverInfo *models.ServerInfoModel) {
	msgStr := ""
	appendStr := func(str string) {
		if msgStr != "" {
			msgStr += "\n" + str
		} else {
			msgStr = str
		}
	}

	//检测内存
	if sshSession, sessionErr := models.NewOnceSSHSession(serverInfo, 22); sessionErr == nil {
		defer func() { _ = sshSession.MyClose() }()
		if memoryInfo, err := GetServerMemoryInfo(sshSession); err == nil && memoryInfo != nil && memoryInfo.UsedPercent >= float64(ServerMemoryMax) {
			appendStr("物理机:" + serverInfo.ServerName + memoryInfo.ToString() + "已超出指定阈值!")
		}
	}

	//检测CPU
	if sshSession, sessionErr := models.NewOnceSSHSession(serverInfo, 22); sessionErr == nil {
		defer func() { _ = sshSession.MyClose() }()
		if CPUInfo, err := GetServerCPUInfo(sshSession); err == nil && CPUInfo >= float64(ServerCPUMax) {
			appendStr(fmt.Sprintf("物理机:%v CPU使用率:%.1f%%", serverInfo.ServerName, CPUInfo) + "已超出指定阈值!")
		}
	}

	//检测硬盘
	if sshSession, sessionErr := models.NewOnceSSHSession(serverInfo, 22); sessionErr == nil {
		defer func() { _ = sshSession.MyClose() }()
		if diskInfoList, err := GetServerDiskInfo(sshSession); err == nil && len(diskInfoList) > 0 {
			for _, item := range diskInfoList {
				if item.UsedPercent >= float64(ServerDiskMax) {
					appendStr("物理机:" + serverInfo.ServerName + item.ToString() + "已超出指定阈值!")
				}
			}
		}
	}

	if msgStr == "" {
		return
	}

	msgStr = "注意:\n" + msgStr
	SendDingMsg(msgStr, models.GetManagerPhones())
}

//定时检测服务器是否正常运行
func TimerCheckServerRun() {
	if isCheckServerRun {
		nowTime := time.Now().Unix()
		//获取所有物理机
		serverInfoList := models.GetAllServerInfo()

		//一定要先拉起跨服服务器（如果挂了）
		for _, item := range serverInfoList {
			if item.ServerName == "HubServer" {
				checkOtherServerRun(item)
				break
			}
		}

		//再检查其他服务
		for _, item := range serverInfoList {
			//检查中心服
			if item.ServerName == "SLGPaymentWeb" {
				go func(info *models.ServerInfoModel) {
					defer func() {
						if r := recover(); r != nil {
							log.Error("r=", r)
						}
					}()

					checkOtherServerRun(info)
				}(item)

				continue
			}

			//获取该物理机上安装的所有服务器
			serverConfList := models.GetAllServerConf(item.ServerName)
			for _, tempServerConf := range serverConfList {
				//判断是否新服（5固定新服）或者是已开服;允许1分钟误差
				if tempServerConf.ServerType == 5 || nowTime < tempServerConf.StartTime.Unix()+60 {
					continue
				}

				//检查服务器是否正在运行
				go func(info *models.ServerInfoModel, conf *models.ServerConfig) {
					defer func() {
						if r := recover(); r != nil {
							log.Error("r=", r)
						}
					}()

					checkServerRun(info, conf)
				}(item, tempServerConf)
			}
		}
	}

	time.AfterFunc(time.Duration(GameServerTimerCheckInterval)*time.Second, TimerCheckServerRun)
}

//检查服务器是否在运行,服务器挂了则自动拉起
func checkServerRun(serverInfo *models.ServerInfoModel, serverConf *models.ServerConfig) {
	sshSession, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		log.Error("获取物理机的SSH失败!err=" + err.Error())
		return
	}
	defer func() { _ = sshSession.MyClose() }()

	checkCommand := fmt.Sprintf("ps -ef|grep 'SLGServer %v'|grep -v 'grep' && echo yes || echo no", serverConf.ServerID)
	if err := sshSession.Run(checkCommand); err != nil {
		log.Error(fmt.Sprintf("执行指令:%v出错!err=%v", checkCommand, err.Error()))
		return
	}

	if strings.Contains(sshSession.OutToString(), "yes") {
		return
	}

	result := fmt.Sprintf("游戏服[%v|%v]挂了!", serverConf.ServerID, serverConf.ServerName)
	defer func() { SendDingMsg(result, models.GetManagerPhones()) }()

	restartSession, err2 := models.NewOnceSSHSession(serverInfo, 22)
	if err2 != nil {
		result += fmt.Sprintf("获取物理机[%v|%v]session失败!重启游戏服失败!", serverInfo.ServerName, serverInfo.InnerIP)
		return
	}
	defer func() { _ = restartSession.MyClose() }()

	//服务器挂了,先备份下日志,然后再拉起来
	restartCommand := fmt.Sprintf("cd /data/gamesvr/;mkdir backuplog;cp output_%v ./backuplog/output_%v_%v;./start.sh %v",
		serverConf.ServerID, serverConf.ServerID, tool.GetTimeStringByUTC(time.Now().Unix()), serverConf.ServerID)
	if err := restartSession.Run(restartCommand); err != nil {
		result = fmt.Sprintf("执行指令[%v]失败!重启游戏服失败!", restartCommand)
		return
	}
	result += "服务器重启成功!"
}

//检查中心服和跨服,服务器挂了则自动拉起
func checkOtherServerRun(serverInfo *models.ServerInfoModel) {
	sshSession, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		log.Error("获取物理机的SSH失败!err=" + err.Error())
		return
	}
	defer func() { _ = sshSession.MyClose() }()

	checkCommand := fmt.Sprintf("ps -ef|grep '%v'|grep -v 'grep' && echo yes || echo no", serverInfo.ServerName)
	if err := sshSession.Run(checkCommand); err != nil {
		log.Error(fmt.Sprintf("执行指令:%v出错!err=%v", checkCommand, err.Error()))
		return
	}

	if strings.Contains(sshSession.OutToString(), "yes") {
		return
	}

	result := fmt.Sprintf("服务器[%v]挂了!", serverInfo.ServerName)
	defer func() { SendDingMsg(result, models.GetManagerPhones()) }()

	restartSession, err2 := models.NewOnceSSHSession(serverInfo, 22)
	if err2 != nil {
		result += fmt.Sprintf("获取物理机[%v|%v]session失败!重启服务器失败!", serverInfo.ServerName, serverInfo.InnerIP)
		return
	}
	defer func() { _ = restartSession.MyClose() }()

	//服务器挂了,先备份下日志,然后再拉起来
	restartCommand := fmt.Sprintf("cd /data/%v/;cp output output_%v;./start.sh",
		serverInfo.ServerName, tool.GetTimeStringByUTC(time.Now().Unix()))
	if err := restartSession.Run(restartCommand); err != nil {
		result = fmt.Sprintf("执行指令[%v]失败!重启服务器失败!", restartCommand)
		return
	}
	result += "服务器重启成功!"
}

//定时检查数据库服务是否正常运行
func TimerCheckMongodbRun() {
	//获取所有物理机
	serverInfoList := models.GetAllServerInfo()
	for _, item := range serverInfoList {
		go func(i *models.ServerInfoModel) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("r=", r)
				}
			}()

			//未配置DB信息
			if i.DBPath == "" {
				return
			}

			checkMongodbRun(i)
		}(item)
	}

	time.AfterFunc(time.Duration(MongodbTimerCheckInterval)*time.Second, TimerCheckMongodbRun)
}

//检查数据库服务
func checkMongodbRun(serverInfo *models.ServerInfoModel) {
	sshSession, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		log.Error("获取物理机的SSH失败!err=" + err.Error())
		return
	}
	defer func() { _ = sshSession.MyClose() }()

	checkCommand := "ps -ef|grep 'mongod'|grep -v 'grep' && echo yes || echo no"
	if err3 := sshSession.Run(checkCommand); err3 != nil {
		log.Error("执行指令:%v出错!err=", err3.Error())
		return
	}
	if strings.Contains(sshSession.OutToString(), "yes") {
		return
	}

	result := fmt.Sprintf("Mongodb服务[%v]挂了!", serverInfo.ServerName)
	defer func() { SendDingMsg(result, models.GetManagerPhones()) }()

	restartSession, err4 := models.NewOnceSSHSession(serverInfo, 22)
	if err4 != nil {
		result += fmt.Sprintf("获取物理机[%v|%v]session失败!重启Mongodb服务失败!", serverInfo.ServerName, serverInfo.InnerIP)
		return
	}
	defer func() { _ = restartSession.MyClose() }()

	//Mongodb服务挂了,直接拉起来
	restartCommand := fmt.Sprintf("cd %v;./start.sh", serverInfo.DBPath)
	if err := restartSession.Run(restartCommand); err != nil {
		result += fmt.Sprintf("执行指令[%v]失败!重启Mongodb服务失败!", restartCommand)
		return
	}
	result += "Mongodb服务重启成功!"
}

//设置定时检查游戏服状态
func SetCheckServerRunStatus(isOpen bool) {
	isCheckServerRun = isOpen
}

//定时全量备份
func TimerBackupDBAllData() {
	if AllBackupDir == "" {
		SendDingMsg("未配置数据库全量备份目录!DB全量备份失败!", models.GetManagerPhones())
		return
	}
	if BackupScriptDir == "" || AllBackupScriptName == "" {
		SendDingMsg("未配置数据库全量脚本路径或名称!DB全量备份失败!", models.GetManagerPhones())
		return
	}

	nowTime := time.Now()
	//构造今天全量备份的路径
	dirName := AllBackupDir + fmt.Sprintf("%d%02d%02d", nowTime.Year(), nowTime.Month(), nowTime.Day())

	//获取从库所在的物理机
	serverInfo := GetSecondaryDBServer()
	if serverInfo == nil {
		SendDingMsg("未配置从库地址.数据库备份失败!", models.GetManagerPhones())
		return
	}

	//判断今天有没有进行全量备份
	if isExists, err := serverInfo.CheckPathIsExists(dirName); err == nil {
		//目录不存在表示今天未进行备份,启动协程进行备份
		if !isExists {
			tool.GoFunc(func() {
				result := StartBackupDBAllData()
				result += "\n数据库全量备份完成!"
				SendDingMsg(result, models.GetManagerPhones())
			})
		}
	} else {
		SendDingMsg(err.Error()+"\n数据库备份失败!", models.GetManagerPhones())
	}

	runTime := tool.GetBeginTime(nowTime.Unix()) + 3600*24 + 3600*int64(AllBackupTime) //计算到明天凌晨时间戳
	intervalTime := runTime - nowTime.Unix()                                           //计算间隔时间
	time.AfterFunc(time.Duration(intervalTime)*time.Second, TimerBackupDBAllData)      //启动定时器

	log.Info(fmt.Sprintf("启动定时器,%d秒后进行全量备份", intervalTime))
}

//开始全量备份
func StartBackupDBAllData() (result string) {
	//获取从库所在的物理机信息.获取该物理机的session
	serverInfo := GetSecondaryDBServer()
	if serverInfo == nil {
		result = "未配置从库地址.数据库备份失败!"
		return
	}

	if models.CenterDB == nil {
		result = "链接中心服数据库失败"
	}

	serverConfList := make([]*models.ServerConfig, 0)
	if err := models.CenterDB.GetTableDataAll("Center", "ServerConfig", &serverConfList); err != nil {
		result = "获取所有的服务器配置失败!err=" + err.Error()
		return
	}

	//所有需要备份的数据库名
	backupDBName := make([]string, 0)
	//todo 考虑合服的情况...合过服的DB就不需要进行备份了
	for _, tempConf := range serverConfList {
		backupDBName = append(backupDBName, tempConf.DBName)
	}
	//额外还需要备份的数据库
	if extraDBName := strings.Split(AllBackupDBName, "|"); len(extraDBName) > 0 {
		backupDBName = append(backupDBName, extraDBName...)
	}

	for index, tempDBName := range backupDBName {
		//10个库的备份日志发一次钉钉
		if index%10 == 0 && index != 0 {
			SendDingMsg(result, models.GetManagerPhones())
			result = ""
		}

		//执行备份
		result += RunBackupDBAllData(serverInfo, tempDBName)

		//间隔一会
		time.Sleep(10 * time.Second)
	}

	return
}

//备份DB
func RunBackupDBAllData(serverInfo *models.ServerInfoModel, dbName string) (result string) {
	session, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		result += fmt.Sprintf("获取物理机[%v|%v]session失败!数据库[%v]备份失败!", serverInfo.ServerName, serverInfo.InnerIP, dbName)
		return
	}
	defer func() { _ = session.MyClose() }()

	ip, port := tool.FormatDBAddr(serverInfo.DBAddr)
	commandParam := fmt.Sprintf("'%v' %v %v '%v' %v", ip, port, serverInfo.DBAccount, serverInfo.DBPsd, dbName)
	commandTxt := fmt.Sprintf("cd %s;chmod +x %s;./%s %s", BackupScriptDir, AllBackupScriptName, AllBackupScriptName, commandParam)
	if err := session.Run(commandTxt); err == nil {
		result += session.OutToString() + "\n"
	} else {
		result += fmt.Sprintf("备份数据库%v失败!err=%v", dbName, err.Error())
	}

	return
}

//定时增量备份
func TimerBackupIncrementalData() {
	if BackupScriptDir == "" || IncrementalBackScriptName == "" {
		SendDingMsg("未配置数据库增量脚本路径或名称!DB增量备份失败!", models.GetManagerPhones())
		return
	}

	//获取从库所在的物理机信息.获取该物理机的session
	serverInfo := GetSecondaryDBServer()
	if serverInfo == nil {
		SendDingMsg("未配置从库地址.DB增量备份失败!", models.GetManagerPhones())
		return
	}
	session, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		SendDingMsg(fmt.Sprintf("获取物理机[%v|%v]session失败!DB增量备份失败!", serverInfo.ServerName, serverInfo.InnerIP), models.GetManagerPhones())
		return
	}
	defer func() { _ = session.MyClose() }()

	//直接进行增量备份
	ip, port := tool.FormatDBAddr(serverInfo.DBAddr)
	commandParam := fmt.Sprintf("'%v' %v %v '%v'", ip, port, serverInfo.DBAccount, serverInfo.DBPsd)
	commandTxt := fmt.Sprintf("cd %s;chmod +x %s;./%s %s", BackupScriptDir, IncrementalBackScriptName, IncrementalBackScriptName, commandParam)

	var errMsg string
	if err := session.Run(commandTxt); err == nil {
		errMsg = session.OutToString()
		if !strings.Contains(errMsg, "Fatal Error") {
			errMsg = ""
		}
	} else {
		errMsg += fmt.Sprintf("DB增量备份失败!err=%v", err.Error())
	}

	//发生了错误才需要通知到钉钉
	if errMsg != "" {
		SendDingMsg(errMsg, models.GetManagerPhones())
	}

	time.AfterFunc(time.Duration(3600)*time.Second, TimerBackupIncrementalData) //启动定时器,1小时后再次执行
}

//获取定时检查游戏服状态
func GetCheckServerRunStatus() bool {
	return isCheckServerRun
}

//指令:更新物理机信息
func commFuncUpdateServer(command models.RobotCommand) string {
	if command.CommandParams == "" {
		//如果为空则列出所有物理机数据
		result := "修改物理机信息使用json字符串 如{\"ServerName\":\"center\",\"IP\":\"127.0.0.1\",\"InnerIP\":\"127.0.0.1\",\"UserName\":\"root\",\"Password\":\"123456\",\"DBPath\":\"/data/mongodb/\",\"DBAddr\":\"127.0.0.1:27017\",\"DBAccount\":\"root\",\"DBPsd\":\"123456\"}\n" +
			"若用户名和密码同时为空则表示要删除物理机信息如{\"ServerName\":\"center\",\"IP\":\"\",\"InnerIP\":\"\",\"UserName\":\"\",\"Password\":\"\",\"DBPath\":\"\",\"DBAddr\":\"\",\"DBAccount\":\"\",\"DBPsd\":\"\"}\n"
		return result + models.GetAllServerInfoStr()
	}

	return models.UpdateServerInfo(command.CommandParams)
}

//指令:获取物理机内存
func commFuncServerMemoryGet(command models.RobotCommand) string {
	var result string

	//默认取所有物理机的内存
	allServerInfo := models.GetAllServerInfo()
	for _, tempServerInfo := range allServerInfo {
		//判断是否指定了物理机
		if command.CommandParams != "" && command.CommandParams != tempServerInfo.ServerName {
			continue
		}

		sshSession, err := models.NewOnceSSHSession(tempServerInfo, 22)
		if err != nil {
			result += fmt.Sprintf(fmt.Sprintf("\n连接物理机[%v]失败!serverInfo=%+v err=%v", tempServerInfo.ServerName, tempServerInfo, err.Error()))
			continue
		}
		memoryInfo, err2 := GetServerMemoryInfo(sshSession)
		_ = sshSession.MyClose()
		if err2 != nil {
			result += fmt.Sprintf("\n获取物理机[%v]内存信息失败!err=%v", tempServerInfo.ServerName, err2.Error())
			continue
		}

		result += fmt.Sprintf("\n物理机[%v]内存信息:%v", tempServerInfo.ServerName, memoryInfo.ToString())
	}

	if command.CommandParams != "" && result == "" {
		result = fmt.Sprintf("物理机名称:%v未配置!", command.CommandParams)
	}

	return result
}

//指令:获取物理机CPU
func commFuncServerCPUGet(command models.RobotCommand) string {
	var result string

	//默认取所有物理机CPU.
	wg := sync.WaitGroup{}
	allServerInfo := models.GetAllServerInfo()
	for _, tempServerInfo := range allServerInfo {
		//判断是否指定了物理机
		if command.CommandParams != "" && command.CommandParams != tempServerInfo.ServerName {
			continue
		}

		wg.Add(1)
		go func(serverInfo *models.ServerInfoModel) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("r=", r)
				}
				wg.Done()
			}()

			sshSession, err := models.NewOnceSSHSession(serverInfo, 22)
			if err != nil {
				result += fmt.Sprintf(fmt.Sprintf("\n连接物理机[%v]失败!serverInfo=%+v err=%v", serverInfo.ServerName, serverInfo, err.Error()))
				return
			}
			cupValue, err2 := GetServerCPUInfo(sshSession)
			_ = sshSession.MyClose()
			if err2 != nil {
				result += fmt.Sprintf("\n获取物理机[%v]CPU信息失败!err=%v", serverInfo.ServerName, err2.Error())
				return
			}

			result += fmt.Sprintf("\n物理机[%v]CPU使用率:%.1f%%", serverInfo.ServerName, cupValue)
		}(tempServerInfo)
	}
	wg.Wait()

	if command.CommandParams != "" && result == "" {
		result = fmt.Sprintf("物理机名称:%v未配置!", command.CommandParams)
	}

	return result
}

//指令:获取物理机硬盘
func commFuncServerDiskInfoGet(command models.RobotCommand) string {
	var result string

	//默认取所有物理机硬盘
	allServerInfo := models.GetAllServerInfo()
	for _, tempServerInfo := range allServerInfo {
		//判断是否指定了物理机
		if command.CommandParams != "" && command.CommandParams != tempServerInfo.ServerName {
			continue
		}

		sshSession, err := models.NewOnceSSHSession(tempServerInfo, 22)
		if err != nil {
			result += fmt.Sprintf(fmt.Sprintf("\n连接物理机[%v]失败!serverInfo=%+v err=%v", tempServerInfo.ServerName, tempServerInfo, err.Error()))
			continue
		}
		diskInfoList, err2 := GetServerDiskInfo(sshSession)
		_ = sshSession.MyClose()
		if err2 != nil {
			result += fmt.Sprintf("\n获取物理机[%v]硬盘信息失败!err=%v", tempServerInfo.ServerName, err2.Error())
			continue
		}

		var resultStr string
		for _, item := range diskInfoList {
			resultStr += item.ToString() + "\n"
		}

		result += fmt.Sprintf("\n物理机[%v]硬盘(大于10G)使用情况:\n%v", tempServerInfo.ServerName, resultStr)
	}

	if command.CommandParams != "" && result == "" {
		result = fmt.Sprintf("物理机名称:%v未配置!", command.CommandParams)
	}

	return result
}

//指定:获取物理机概要信息
func commFuncServerAllInfoGet(command models.RobotCommand) string {
	var result string

	//默认取所有物理机硬盘
	allServerInfo := models.GetAllServerInfo()
	for _, tempServerInfo := range allServerInfo {
		//判断是否指定了物理机
		if command.CommandParams != "" && command.CommandParams != tempServerInfo.ServerName {
			continue
		}

		result += commFuncServerMemoryGet(models.RobotCommand{CommandParams: tempServerInfo.ServerName})
		result += commFuncServerCPUGet(models.RobotCommand{CommandParams: tempServerInfo.ServerName})
		result += commFuncServerDiskInfoGet(models.RobotCommand{CommandParams: tempServerInfo.ServerName})

		result += "\n********************************************************************************\n"
	}

	if command.CommandParams != "" && result == "" {
		result = fmt.Sprintf("物理机名称:%v未配置!", command.CommandParams)
	}

	return result
}

//指令:重启服务器
func commFuncResetGameServer(command models.RobotCommand) string {
	paramList := strings.Split(command.CommandParams, ":")
	if len(paramList) != 2 {
		return "参数数量错误!命令格式【重启游戏服:物理机名称:服务器所在文件夹名称】例如:【重启游戏服:gameServer1:gamesvr10001】【重启中心服:center:SLGPaymentWeb】"
	}

	serverInfo := models.GetServerInfoByServerName(paramList[0])
	if serverInfo == nil {
		return fmt.Sprintf("物理机名称:%v未配置!", command.CommandParams)
	}

	//构造指令
	stopCommandTxt := fmt.Sprintf("cd %s;chmod +x %s.sh;./%s.sh", "/data/"+paramList[1], "stop", "stop")
	startCommandTxt := fmt.Sprintf("cd %s;chmod +x %s.sh;./%s.sh", "/data/"+paramList[1], "start", "start")

	//关闭服务器
	stopSession, StopErr := models.NewOnceSSHSession(serverInfo, 22)
	if StopErr != nil {
		return fmt.Sprintf("连接物理机失败!serverInfo=%+v,err=%v", serverInfo, StopErr.Error())
	}
	defer func() { _ = stopSession.MyClose() }()
	if runErr := stopSession.Run(stopCommandTxt); runErr != nil {
		return fmt.Sprintf("执行关闭服务器指令出错!command=%v err=%v", stopCommandTxt, runErr.Error())
	}

	//开启服务器
	startSession, startErr := models.NewOnceSSHSession(serverInfo, 22)
	if startErr != nil {
		return fmt.Sprintf("执行关闭服务器指令成功outResult=%v;\n执行启动服务器指令的时候连接物理机失败!serverInfo=%+v,err=%v", stopSession.OutToString(), serverInfo, startErr.Error())
	}
	defer func() { _ = startSession.MyClose() }()
	if runErr := startSession.Run(startCommandTxt); runErr != nil {
		return fmt.Sprintf("执行关闭服务器指令成功outResult=%v;\n执行开启服务器指令出错!command=%v err=%v", stopSession.OutToString(), startCommandTxt, runErr.Error())
	}

	return stopSession.OutToString() + "\n" + startSession.OutToString() + "\n重启服务器成功"
}

//指令:刷新所有服务器配置
func commFuncUpdateAllServerConf(command models.RobotCommand) string {
	if err := models.InitAllServerConfigData(); err != nil {
		return "刷新所有服务器配置失败!err=" + err.Error()
	}
	return "刷新所有服务器配置成功"
}

//返回物理机内存使用情况
func GetServerMemoryInfo(sshSession *models.SSHSession) (*models.ServerMemoryInfo, error) {
	if err := sshSession.Run("free|grep Mem"); err != nil {
		return nil, err
	}

	//解析结果
	outStr := sshSession.OutToString()
	outStr = outStr[0 : len(outStr)-1]
	realValueList := make([]int, 0)
	outList := strings.Split(outStr, " ")
	for _, item := range outList {
		if item == "" {
			continue
		}

		if value := tool.String2Int64(item); value != 0 {
			realValueList = append(realValueList, int(value))
		}
	}
	if len(realValueList) < 6 {
		errStr := fmt.Sprintf("解析结果失败!out=%v", outStr)
		return nil, errors.New(errStr)
	}

	//返回值单位是KB,这里要转成B所以需要乘上1024
	data := &models.ServerMemoryInfo{
		Total:       tool.ChangByteValue(float64(realValueList[0]) * 1024),
		Available:   tool.ChangByteValue(float64(realValueList[5]) * 1024),
		Used:        tool.ChangByteValue(float64(realValueList[1]) * 1024),
		UsedPercent: float64(realValueList[1]) / float64(realValueList[0]) * 100,
	}

	return data, nil
}

//返回物理机CPU使用率
func GetServerCPUInfo(sshSession *models.SSHSession) (float64, error) {
	// -n 3:更新3次; -d 1:更新间隔1s
	if err := sshSession.Run("top -b -n 3 -d 1|grep Cpu"); err != nil {
		return 0, err
	}

	//输出格式 us:用户占用 sy:系统占用
	//%Cpu(s):  2.0 us,  1.0 sy,  0.0 ni, 97.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
	outStr := sshSession.OutToString()
	outList := strings.Split(outStr, "%Cpu(s):")

	var totalCpu float64
	for _, itemStr := range outList {
		if itemStr == "" {
			continue
		}
		itemList := strings.Split(itemStr, ",")

		for _, tempStr := range itemList {
			if index := strings.Index(tempStr, "us"); index != -1 {
				usCpu := strings.Trim(tempStr[:index], " ")
				totalCpu += tool.String2Float64(usCpu)
			}
			if index := strings.Index(tempStr, "sy"); index != -1 {
				syCpu := strings.Trim(tempStr[:index], " ")
				totalCpu += tool.String2Float64(syCpu)
			}
		}
	}

	//因为是更新了3次,所以这里取3次的平均值
	return totalCpu / 3, nil
}

//返回物理机硬盘使用情况
func GetServerDiskInfo(sshSession *models.SSHSession) ([]*models.ServerDiskInfo, error) {
	if err := sshSession.Run("df -h"); err != nil {
		return nil, err
	}

	diskInfoList := make([]*models.ServerDiskInfo, 0)

	outStr := sshSession.OutToString()
	outList := strings.Split(outStr, "\n")
	for index, itemStr := range outList {
		//过滤表头
		if index == 0 || itemStr == "" {
			continue
		}

		//获取有效数据
		realItemList := make([]string, 0)
		itemList := strings.Split(itemStr, " ")
		for _, tempStr := range itemList {
			if tempStr == "" {
				continue
			}
			realItemList = append(realItemList, tempStr)
		}
		//数据不正确?
		if len(realItemList) < 6 {
			continue
		}

		diskInfo := &models.ServerDiskInfo{
			Filesystem: realItemList[0],
			Path:       realItemList[5],
			Total:      realItemList[1],
			Available:  realItemList[3],
			Used:       realItemList[2],
		}
		if pos := strings.Index(realItemList[4], "%"); pos != -1 {
			diskInfo.UsedPercent = tool.String2Float64(realItemList[4][:pos])
		}

		//diskInfo.Total格式:100G 分割出100和G,过滤10G以下的硬盘信息
		value := diskInfo.Total[:len(diskInfo.Total)-1]
		valueUnit := diskInfo.Total[len(diskInfo.Total)-1:]
		//应该不会挂载TB这么大的硬盘吧?
		if valueUnit != "G" && valueUnit != "T" {
			continue
		}
		if valueUnit == "G" && tool.String2Float64(value) < 10 {
			continue
		}

		diskInfoList = append(diskInfoList, diskInfo)
	}

	return diskInfoList, nil
}

//获取从库所在的物理机
func GetSecondaryDBServer() *models.ServerInfoModel {
	allDBServer := make([]*models.ServerInfoModel, 0)

	//先取到所有DB所在的物理机
	serverInfoList := models.GetAllServerInfo()
	for _, item := range serverInfoList {
		if item.DBPath != "" {
			allDBServer = append(allDBServer, item)
		}
	}

	if len(allDBServer) <= 0 {
		return nil
	}

	if len(allDBServer) == 1 {
		SendDingMsg("未配置从库DB?为了保证数据备份能成功,直接返回任意一个DB所在的物理机", models.GetManagerPhones())
		return allDBServer[0]
	}

	//寻找从库
	for _, item := range allDBServer {
		if !item.IsMaster {
			return item
		}
	}

	SendDingMsg("寻找从库DB失败?为了保证数据备份能成功,直接返回任意一个DB所在的物理机", models.GetManagerPhones())
	return allDBServer[0]
}

//判断是否是主库.通过ssh连接物理机.执行对应的脚本
func isMasterDBServer(serverInfo *models.ServerInfoModel) (ok bool, isMaster bool) {
	session, err := models.NewOnceSSHSession(serverInfo, 22)
	if err != nil {
		return
	}
	defer func() { _ = session.MyClose() }()

	ip, port := tool.FormatDBAddr(serverInfo.DBAddr)
	commandParam := fmt.Sprintf("%s %s %s %s", ip, port, serverInfo.DBAccount, serverInfo.DBPsd)
	commandTxt := fmt.Sprintf("cd %s;chmod +x %s;./%s %s", BackupScriptDir, IsMasterMongoScriptName, IsMasterMongoScriptName, commandParam)
	if err2 := session.Run(commandTxt); err2 != nil {
		return
	}

	//标识指令执行成功
	ok = true

	result := session.OutToString()
	if strings.Contains(result, "true") {
		isMaster = true
	}

	return
}
