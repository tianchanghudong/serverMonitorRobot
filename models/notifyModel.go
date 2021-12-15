package models

import (
	"fmt"
	"strings"
)

type InnerServerSendMsg struct {
	InputCommand string
	Executor     string
}

//游戏服上报监控服务器的数据结构
type NotifyPanicData struct {
	ServerID string
	Short    string
	Line     int
	Content  string
}

func (this *NotifyPanicData) GetLogKey() string {
	//判断是panic还是error
	if !strings.Contains(this.Content, "stack start") && !strings.Contains(this.Content, "stack end") {
		//是error,同个文件的同一行认为是同一条error
		return this.Short + fmt.Sprint(this.Line)
	}

	//是panic
	return this.Content
}

//监控服务器上报钉钉的数据结构
type PanicToDingDing struct {
	LogKey     string
	FromServer map[string]int //key:serverID value:count
	Short      string
	Line       int
	Content    string
}

func (this *PanicToDingDing) ToString() string {
	tempStr := ""
	for serverID, count := range this.FromServer {
		tempStr += fmt.Sprintf("【服务器:%v服,发生次数:%v】", serverID, count)
	}
	return fmt.Sprintf("服务器报错:\n %v【%v line : %v】\n 报错统计:%v", this.Content, this.Short, this.Line, tempStr)
}
