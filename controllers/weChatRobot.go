package controllers

import "github.com/astaxie/beego"

//企业微信自动构建，跟钉钉一样收到指令走manager.RecvCommand逻辑
type WeChatAutoBuld struct {
	beego.Controller
}

func (this *WeChatAutoBuld) Post(){
	this.ServeJSON()
}

func (this *WeChatAutoBuld) Get(){
	this.ServeJSON()
}