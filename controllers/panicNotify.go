package controllers

import (
	"encoding/json"
	"github.com/astaxie/beego"
	"servermonitorrobot/log"
	"servermonitorrobot/manager"
	"servermonitorrobot/models"
)

type PanicNotify struct {
	beego.Controller
}

//服务器挂了上报日志
func (this *PanicNotify) Post() {
	defer func() {
		this.ServeJSON()
	}()

	//解析服务器传过来得数据
	sendMsg := new(models.NotifyPanicData)
	if err := json.Unmarshal(this.Ctx.Input.RequestBody, sendMsg); err != nil {
		result := "解析服务器上报的panic数据异常:" + err.Error()
		log.Error(result)
		return
	}

	manager.GPanicLogManager.SaveLog(sendMsg)
}

func (c *PanicNotify) Get() {
	c.ServeJSON()
}
