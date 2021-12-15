package controllers

import (
	"fmt"
	"github.com/astaxie/beego"
	"servermonitorrobot/manager"
)

type ServerRunCheck struct {
	beego.Controller
}


//因为服务器挂了，监控会自动重启，但是维护的时候就不能重启，所以这是后台发消息关闭监控
//status=0:查看状态 status=1:启动监控 status=2:关闭监控
func (this *ServerRunCheck) Post() {
	status := this.Input().Get("status")
	switch status {
	case "0":
		var resultMsg string
		if manager.GetCheckServerRunStatus() {
			resultMsg = fmt.Sprintf("{\"Status\":\"%v\"}", 1)
		} else {
			resultMsg = fmt.Sprintf("{\"Status\":\"%v\"}", 2)
		}
		this.Data["json"] = resultMsg
	default:
		manager.SetCheckServerRunStatus(status == "1")
		this.Data["json"] = fmt.Sprintf("{\"Status\":\"%v\"}", status)
	}

	this.ServeJSON()
}

func (c *ServerRunCheck) Get() {
	c.ServeJSON()
}
