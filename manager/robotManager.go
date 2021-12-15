package manager

import (
	"fmt"
	"github.com/astaxie/beego"
	"runtime"
	"servermonitorrobot/log"
	"servermonitorrobot/models"
	"servermonitorrobot/tool"
	"strings"
	"time"
)

//app.conf配置信息
var winGitPath = ""     //window git 安装路径，用于执行shell脚本
var shellPath = "shell" //shell脚本地址
var lineInOneMes = 80   //一条构建消息的行数

var DingDingRobotAppSecret = "" //钉钉机器人密钥
var DingDingWebHook = ""        //回调webhook

var ServerMemoryMax int              //内存阈值
var ServerCPUMax int                 //cpu阈值
var ServerDiskMax int                //硬盘阈值
var ServerTimerCheckInterval int     //定时检测服务器信息间隔(单位:秒)
var GameServerTimerCheckInterval int //游戏服定时检测间隔(单位秒)
var MongodbTimerCheckInterval int    //数据库定时检测间隔(单位秒)

var PanicLogSendInterval int //错误日志定时发送间隔(单位秒)
var PanicLogCacheTime int    //错误日志的缓存时长(单位秒)

var centerDBAddr string //中心服数据库地址

var AllBackupTime int                //数据库全量备份时间
var AllBackupDir string              //数据库全量备份目录
var BackupScriptDir string           //数据库备份脚本路径
var AllBackupScriptName string       //数据库全量备份脚本名称
var IncrementalBackScriptName string //数据库增量备份脚本名称
var IsMasterMongoScriptName string   //检测是否为主库脚本名称
var AllBackupDBName string           //除了游戏库以外,需要全量备份的数据库;用"|"进行分割
var CheckServerRunPath string        //需要检查服务器是否运行的文件/目录;用"|"进行分割;游戏服目录放在第一个!!!

//初始化
func init() {
	initConfData()

	GPanicLogManager.Init()

	//初始化指令、指令名字、指令处理函数
	models.AddCommand(models.CommandType_Help, "", "帮助", helpCommand)
	models.AddCommand(models.CommandType_UpdateUser, "", "更新用户", updateUserCommand)
	models.AddCommand(models.CommandType_UpdateServer, "", "更新物理机信息", commFuncUpdateServer)
	//models.AddCommand(models.CommandType_GetServerMemory, "", "获取物理机内存信息", commFuncServerMemoryGet)
	//models.AddCommand(models.CommandType_GetServerCPU, "", "获取物理机CPU信息", commFuncServerCPUGet)
	//models.AddCommand(models.CommandType_GetServerDisk, "", "获取物理机硬盘信息", commFuncServerDiskInfoGet)
	models.AddCommand(models.CommandType_GetServerAllInfo, "", "获取物理机概要信息", commFuncServerAllInfoGet)
	models.AddCommand(models.CommandType_ResetServer, "", "重启游戏服", commFuncResetGameServer)
	models.AddCommand(models.CommandType_GetPanicLog, "", "获取服务器报错日志", commFuncPanicLogGet)
	models.AddCommand(models.CommandType_UpdateNewServerConf, "", "更新新服配置", commFuncUpdateNewServerConfData)
	models.AddCommand(models.CommandType_AddNewServer, "", "增加新服", commFuncAddNewServer)
	models.AddCommand(models.CommandType_UpdateAllServerConf, "", "刷新所有服务器配置", commFuncUpdateAllServerConf)

	//初始化中心服链接
	if err := models.InitCenterDB(centerDBAddr); err != nil {
		SendDingMsg("初始化CenterDB出错!err="+err.Error(), models.GetManagerPhones())
	}

	//初始化所有的服务器配置
	if err := models.InitAllServerConfigData(); err != nil {
		SendDingMsg("初始化ServerConfig出错!err="+err.Error(), models.GetManagerPhones())
	}

	time.AfterFunc(time.Duration(ServerTimerCheckInterval)*time.Second, TimerCheckServerInfo)
	time.AfterFunc(time.Duration(GameServerTimerCheckInterval)*time.Second, TimerCheckServerRun)
	time.AfterFunc(time.Duration(MongodbTimerCheckInterval)*time.Second, TimerCheckMongodbRun)

	//启动定时全量备份数据库
	TimerBackupDBAllData()
	//启动定时增量备份数据库
	TimerBackupIncrementalData()
}

//初始化conf里面的配置数据
func initConfData() {
	if temp, err := beego.GetConfig("String", "winGitPath", ""); err == nil {
		winGitPath = temp.(string)
	}
	if temp, err := beego.GetConfig("Int", "lineInOneMes", 80); err == nil {
		lineInOneMes = temp.(int)
	}
	log.Info(fmt.Sprintf("winGitPath:%s,lineInOneMes：%d", winGitPath, lineInOneMes))

	if temp, err := beego.GetConfig("String", "dingdingRobotAppSecret", ""); err == nil {
		DingDingRobotAppSecret = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "dingdingWebHook", ""); err == nil {
		DingDingWebHook = temp.(string)
	}
	if temp, err := beego.GetConfig("Int", "serverMemoryMax", 70); err == nil {
		ServerMemoryMax = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "serverCPUMax", 70); err == nil {
		ServerCPUMax = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "serverDiskMax", 80); err == nil {
		ServerDiskMax = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "serverTimerCheckInterval", 300); err == nil {
		ServerTimerCheckInterval = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "gameServerTimerCheckInterval", 60); err == nil {
		GameServerTimerCheckInterval = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "mongodbTimerCheckInterval", 60); err == nil {
		MongodbTimerCheckInterval = temp.(int)
	}

	if temp, err := beego.GetConfig("Int", "panicLogSendInterval", 300); err == nil {
		PanicLogSendInterval = temp.(int)
	}
	if temp, err := beego.GetConfig("Int", "panicLogCacheTime", 86400); err == nil {
		PanicLogCacheTime = temp.(int)
	}

	if temp, err := beego.GetConfig("String", "centerDBAddr", ""); err == nil {
		centerDBAddr = temp.(string)
	}

	if temp, err := beego.GetConfig("Int", "allBackupTime", 2); err == nil {
		AllBackupTime = temp.(int)
	}
	if temp, err := beego.GetConfig("String", "allBackupDir", ""); err == nil {
		AllBackupDir = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "backupScriptDir", ""); err == nil {
		BackupScriptDir = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "allBackupScriptName", ""); err == nil {
		AllBackupScriptName = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "incrementalBackScriptName", ""); err == nil {
		IncrementalBackScriptName = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "isMasterMongoScriptName", ""); err == nil {
		IsMasterMongoScriptName = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "allBackupDBName", ""); err == nil {
		AllBackupDBName = temp.(string)
	}
	if temp, err := beego.GetConfig("String", "checkServerRunPath", ""); err == nil {
		CheckServerRunPath = temp.(string)
	}
}

//收到指令
func RecvCommand(executor, commandMsg string, sendMsgFunc models.RobotResultFunc) {
	isError := false
	result := fmt.Sprintf("正在执行%s...", commandMsg)
	phoneNum := models.GetUserPhone(executor)

	//处理异常
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Errorf("程序异常:%v,大概率网络异常，重试一次试试！", r).Error()
			sendMsgFunc(fmt.Sprintf("builder:%s\ninfo:%s", executor, result), phoneNum)
		}
	}()

	//解析指令
	ok, autoBuildCommand := models.AnalysisCommand(commandMsg)
	autoBuildCommand.Executor = executor
	autoBuildCommand.ResultFunc = sendMsgFunc
	if !ok {
		result = "不存在指令！请输入正确指令：\n"
		isError = true
	}

	//判断是否有权限
	isHavePermission, tips := models.JudgeIsHadPermission(executor, autoBuildCommand.CommandType)
	if !isHavePermission {
		result = tips
		phoneNum += "," + models.GetManagerPhones()
		sendMsgFunc(fmt.Sprintf("builder:%s\ninfo:%s", executor, result), phoneNum)
		return
	}

	//先通知构建群操作结果
	phone := ""
	if isError {
		//有错误才要@回操作者
		phone = phoneNum
	}
	sendMsgFunc(fmt.Sprintf("builder:%s\ninfo:%s", executor, result), phone)

	//执行指令
	commandResult := autoBuildCommand.Func(autoBuildCommand)

	//发送执行结果
	sendMsgFunc(fmt.Sprintf("builder:%s\ncommand:%s\ninfo:%s", executor, autoBuildCommand.Name, commandResult), phoneNum)
}

//执行帮助指令
func helpCommand(command models.RobotCommand) string {
	help := "输入指令名字或者编号选择要执行的操作，如果有参数则命令后加冒号和参数，如果缺参数则会输出详细参数指引\n"
	help += models.GetCommandHelpInfo()
	return help
}

//执行shell模板指令
func shellTempCommand(command models.RobotCommand) string {
	//获取指令
	commandTxt := command.Command
	commandParams := command.CommandParams
	commandTxt = fmt.Sprintf("cd %s;chmod +x %s.sh;./%s.sh %s", shellPath, commandTxt, commandTxt, commandParams)
	if commandTxt == "" {
		return "shellCommand,指令为空，请检查！！！"
	}

	return shellCommand(commandTxt, command.ResultFunc)
}

//执行shell指令
func shellCommand(commandTxt string, ResultFunc models.RobotResultFunc) (result string) {
	//执行指令
	temp := ""
	count := 0
	commandName := "sh"
	if runtime.GOOS == "windows" {
		commandName = winGitPath
	}

	tool.ExecCommand(commandName, commandTxt, func(resultLine string) {
		if strings.Contains(resultLine, "执行完毕！") {
			result = temp + "\n" + resultLine
		} else {
			//每隔80行发送一条构建消息
			count++
			temp += resultLine
			if count >= lineInOneMes {
				ResultFunc(temp, "")
				temp = ""
				count = 0
			}
		}
	})

	return
}

//更新用户指令
func updateUserCommand(command models.RobotCommand) (result string) {
	userInfo := command.CommandParams
	if userInfo == "" {
		//如果为空则列出所有用户
		result += "修改用户名字电话权限项目权限以英文逗号分割\n如【更新用户：张三,158xxx,14,xx项目】,如电话不修改则【张三,,14,xx项目】\n" +
			"多个用户用英文分号分割,分配多个权限则用|分割，负数表示删除对应枚举权限,\n添加项目权限直接输项目名字，多个项目权限用|分割\n"
		result += models.GetAllUserInfo()
		return result
	} else {
		//更新用户数据
		result = models.UpdateUserInfo(userInfo)
		return
	}
}

//发送结果到钉钉群
func SendDingMsg(msg, phoneNumList string) {
	//替换svn信息中钉钉认为得敏感信息（会被吞掉）
	msg = strings.Replace(msg, "\r\n", "\n", -1)
	msg = strings.Replace(msg, "\\", "/", -1)
	msg = strings.Replace(msg, "\"", "\\\"", -1)
	content := `{"msgtype": "text",
		"text": {"content": "` + msg + `"},
		"at": {
         "atMobiles": [
            ` + phoneNumList + ` 
         ], 
         "isAtAll": false
     }
	}`

	_error, result := tool.Http("POST", DingDingWebHook, content)
	if _error == nil {
		return
	}
	log.Error("回调钉钉异常：", string(result))
}
