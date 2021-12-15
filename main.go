//功能：自动构建机器人（以钉钉为例，支持拓展其他办公通讯工具）
//设计思路：用beego搭建web服务，在钉钉开发者后台设置web接收地址，@钉钉机器人发送得消息会转发到本web服务，通过接收到得消息执行相应得指令
//主要文件 app.conf为服务得一些配置信息 autoBuildManager.go为主要逻辑代码
//时间：2020/09/04
//作者：lyp
package main

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/plugins/cors"
	_ "servermonitorrobot/routers"
)

func main() {
	//允许跨域请求
	beego.InsertFilter("*", beego.BeforeRouter, cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"Origin", "Authorization", "Access-Control-Allow-Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
	}))

	beego.Run()
}
