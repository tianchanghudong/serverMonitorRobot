package routers

import (
	"github.com/astaxie/beego"
	"servermonitorrobot/controllers"
)

func init() {
	//接收钉钉群指令
	beego.Router("/", &controllers.DingDingController{})

	//接收企业微信群指令（暂未实现）
	beego.Router("/weChat", &controllers.WeChatAutoBuld{})

	//接收服务器挂之前的日志
	beego.Router("/panicNotify", &controllers.PanicNotify{})

	//接收后台指令（是否开启和关闭监控）
	beego.Router("/serverRunCheck", &controllers.ServerRunCheck{})
}
