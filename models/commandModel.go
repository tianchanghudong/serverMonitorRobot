package models

import (
	"fmt"
	"servermonitorrobot/log"
	"strconv"
	"strings"
	"sync"
)

//指令类型
const (
	CommandType_Help                = 0 //帮助
	CommandType_UpdateUser          = 1 //更新用户
	CommandType_UpdateServer        = 2 //更新物理机信息
	CommandType_GetServerAllInfo    = 3 //获取物理机概要信息
	CommandType_ResetServer         = 4 //重启服务器
	CommandType_GetPanicLog         = 5 //获取服务器报错日志
	CommandType_UpdateNewServerConf = 6 //更新新服配置
	CommandType_AddNewServer        = 7 //添加新服
	CommandType_UpdateAllServerConf = 8 //刷新所有服务器配置
	CommandType_Max                 = 9
)

//机器人指令
type RobotCommand struct {
	CommandType   int              //指令类型
	Command       string           //指令
	Name          string           //指令名字
	CommandParams string           //指令参数
	InputCommand  string           //输入指令
	Executor      string           //执行命令者
	Func          robotCommandFunc //指令处理函数
	ResultFunc    RobotResultFunc  //结果处理函数
}

type robotCommandFunc func(autoBuildCommand RobotCommand) string //指令处理函数指针
type RobotResultFunc func(msg, executorPhoneNum string)          //自动构建结果处理函数
var robotCommandMap map[int]RobotCommand
var robotCommandRWLock sync.RWMutex

func init() {
	robotCommandMap = make(map[int]RobotCommand)
}

//添加指令
func AddCommand(commandType int, command, commandName string, commandFunc robotCommandFunc) {
	if commandType < CommandType_Help || commandType >= CommandType_Max {
		log.Error(fmt.Sprintf("添加越界指令，指令范围：%d-%d", CommandType_Help, CommandType_Max))
		return
	}
	if _, ok := robotCommandMap[commandType]; ok {
		log.Error(fmt.Sprintf("添加重复指令：%d,请检查", commandType))
		return
	}
	autoBuildCommand := RobotCommand{}
	autoBuildCommand.CommandType = commandType
	autoBuildCommand.Command = command
	autoBuildCommand.Name = commandName
	autoBuildCommand.Func = commandFunc
	robotCommandMap[commandType] = autoBuildCommand
}

//获取指令
func GetCommand(commandType int) (autoBuildCommand RobotCommand, ok bool) {
	robotCommandRWLock.RLock()
	defer robotCommandRWLock.RUnlock()
	autoBuildCommand, ok = robotCommandMap[commandType]
	return
}

//获取指令帮助信息
func GetCommandHelpInfo() (help string) {
	robotCommandRWLock.RLock()
	defer robotCommandRWLock.RUnlock()
	for i := 0; i < CommandType_Max; i++ {
		command, ok := robotCommandMap[i]
		if !ok {
			errs := fmt.Sprintf("不存在编号为%d的指令，请添加！", i)
			help += errs
			log.Error(errs)
			continue
		}
		help += fmt.Sprintf("%d:%s\n", i, command.Name)
	}
	return
}

//解析指令
func AnalysisCommand(rawCommand string) (ok bool, autoBuildCommand RobotCommand) {
	//解析指令,先分割参数
	paramSeparators := []string{":", "："}
	requestCommand := rawCommand
	requestParam := ""
	separatorIndex := 99999
	for _, v := range paramSeparators {
		//找到第一个包含分隔符的，通过索引比较，避免分割了带分隔符的参数
		tempIndex := strings.Index(rawCommand, v)
		if tempIndex >= 0 && tempIndex < separatorIndex {
			//有参数
			separatorIndex = tempIndex
			commands := strings.SplitN(rawCommand, v, 2)

			//参数去掉空格和换行
			requestCommand = commands[0]
			requestParam = strings.TrimSpace(commands[1])
			requestParam = strings.Replace(requestParam, "\n", "", -1)
		}
	}

	//获取指令信息
	robotCommandRWLock.RLock()
	requestCommand = strings.Replace(requestCommand, " ", "", -1)
	for _, command := range robotCommandMap {
		if strings.Compare(requestCommand, strconv.Itoa(command.CommandType)) == 0 ||
			strings.Compare(requestCommand, command.Name) == 0 {
			autoBuildCommand = command
			ok = true
			break
		}
	}

	//获取指令信息
	if !ok {
		autoBuildCommand = robotCommandMap[CommandType_Help]
	}
	autoBuildCommand.CommandParams = requestParam
	autoBuildCommand.InputCommand = rawCommand
	robotCommandRWLock.RUnlock()
	return
}

//获取指令名字
func GetCommandNameByType(commandType int) string {
	robotCommandRWLock.RLock()
	defer robotCommandRWLock.RUnlock()

	//获取指令名字
	if command, ok := robotCommandMap[commandType]; ok {
		return command.Name
	}
	return "不存在指令类型：" + strconv.Itoa(commandType)
}
