package manager

import (
	"servermonitorrobot/models"
	"sync"
	"time"
)

type PanicLogManager struct {
	AllServerPanicLogMap *sync.Map //key:serverID value:ServerPanicLogManager
	LastSendDingDingTime int64     //上次上报钉钉的时间
}

type ServerPanicLogManager struct {
	sync.Mutex
	PanicLogMap map[string]*PanicLogData
}

type PanicLogData struct {
	ServerID       string
	LogKey         string
	Short          string //游戏服上报的log
	Line           int    //游戏服上报的log
	Content        string //游戏服上报的log
	Count          int    //相同log上报次数:距离上次自动上报钉钉的累计次数
	TotalCount     int    //距离上次清理缓存的累计次数
	LastUpdateTime int64  //每次上报相同log刷新该时间
}

var GPanicLogManager PanicLogManager

func (this *PanicLogManager) Init() {
	this.AllServerPanicLogMap = new(sync.Map)
	this.LastSendDingDingTime = time.Now().Unix()

	time.AfterFunc(time.Duration(PanicLogSendInterval)*time.Second, this.timerSendToDingDing)
	time.AfterFunc(time.Duration(PanicLogCacheTime)*time.Second, this.timerCleanCache)
}

//定时上报错误日志到钉钉
func (this *PanicLogManager) timerSendToDingDing() {
	time.AfterFunc(time.Duration(PanicLogSendInterval)*time.Second, this.timerSendToDingDing)

	//整理log,合并所有服务器的相同log
	sendMsgMap := this.getPanicSendMsg(true)
	if len(sendMsgMap) <= 0 {
		return
	}

	//上报钉钉群
	for _, sendMsg := range sendMsgMap {
		SendDingMsg(sendMsg.ToString(), models.GetManagerPhones())
	}
}

//定时清理缓存
func (this *PanicLogManager) timerCleanCache() {
	time.AfterFunc(time.Duration(PanicLogCacheTime)*time.Second, this.timerCleanCache)

	nowTime := time.Now().Unix()
	this.AllServerPanicLogMap.Range(func(key, value interface{}) bool {
		serverManager := value.(*ServerPanicLogManager)
		serverManager.Lock()
		defer serverManager.Unlock()

		for LogKey, panicLog := range serverManager.PanicLogMap {
			if nowTime-panicLog.LastUpdateTime < int64(PanicLogCacheTime) {
				continue
			}

			delete(serverManager.PanicLogMap, LogKey)
		}

		if len(serverManager.PanicLogMap) <= 0 {
			this.AllServerPanicLogMap.Delete(key)
		}
		return true
	})
}

//指令:获取服务器报错日志
func commFuncPanicLogGet(command models.RobotCommand) string {
	sendMsgMap := GPanicLogManager.getPanicSendMsg(false)
	if len(sendMsgMap) <= 0 {
		return "暂无报错日志"
	}

	resultStr := ""
	for _, sendMsg := range sendMsgMap {
		resultStr += sendMsg.ToString() + "\n\n"
	}

	return resultStr
}

//接受游戏服上报的log
func (this *PanicLogManager) SaveLog(notifyPanicData *models.NotifyPanicData) {
	logKey := notifyPanicData.GetLogKey()

	serverManager := this.getServerManager(notifyPanicData.ServerID)
	serverManager.Lock()
	defer serverManager.Unlock()

	panicLog, ok := serverManager.PanicLogMap[logKey]
	if !ok {
		panicLog = new(PanicLogData)
		panicLog.ServerID = notifyPanicData.ServerID
		panicLog.LogKey = logKey
		panicLog.Short = notifyPanicData.Short
		panicLog.Line = notifyPanicData.Line
		panicLog.Content = notifyPanicData.Content
		panicLog.Count = 1
		panicLog.TotalCount = 1
		panicLog.LastUpdateTime = time.Now().Unix()
	} else {
		panicLog.Count++
		panicLog.TotalCount++
		panicLog.LastUpdateTime = time.Now().Unix()
	}
	serverManager.PanicLogMap[logKey] = panicLog
}

//获取各服务器的log管理器
func (this *PanicLogManager) getServerManager(serverID string) *ServerPanicLogManager {
	serverManager := new(ServerPanicLogManager)
	serverManager.PanicLogMap = make(map[string]*PanicLogData)

	iServerManager, _ := this.AllServerPanicLogMap.LoadOrStore(serverID, serverManager)
	return iServerManager.(*ServerPanicLogManager)
}

//整理log,合并所有服务器的相同log
func (this *PanicLogManager) getPanicSendMsg(isAuto bool) (sendMsgMap map[string]*models.PanicToDingDing) {
	sendMsgMap = make(map[string]*models.PanicToDingDing)

	this.AllServerPanicLogMap.Range(func(key, value interface{}) bool {
		serverManager := value.(*ServerPanicLogManager)
		serverManager.Lock()
		defer serverManager.Unlock()

		for _, panicLog := range serverManager.PanicLogMap {
			if isAuto && panicLog.Count <= 0 {
				continue
			}

			sendMsg, ok := sendMsgMap[panicLog.LogKey]
			if !ok {
				sendMsg = &models.PanicToDingDing{
					LogKey:  panicLog.LogKey,
					Short:   panicLog.Short,
					Line:    panicLog.Line,
					Content: panicLog.Content,
				}
				sendMsg.FromServer = make(map[string]int)
			}

			//是自动上报钉钉群,只需要发送Count;从钉钉群主动获取,那么就需要发送TotalCount
			if isAuto {
				sendMsg.FromServer[panicLog.ServerID] = panicLog.Count
			} else {
				sendMsg.FromServer[panicLog.ServerID] = panicLog.TotalCount
			}

			sendMsgMap[panicLog.LogKey] = sendMsg

			//上报完成要置零
			panicLog.Count = 0
		}
		return true
	})
	return
}
