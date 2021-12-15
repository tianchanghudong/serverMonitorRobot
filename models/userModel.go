package models

import (
	"fmt"
	"github.com/astaxie/beego"
	"math"
	"servermonitorrobot/tool"
	"strconv"
	"strings"
	"sync"
)

var users map[string]*UserModel       //用户数据，key：用户昵称 value:用户数据
var userDataFileName = "userData.gob" //用户数据文件
var superUser = ""                    //负责人
var userDataLock sync.Mutex

//用户数据
type UserModel struct {
	Permission int
	PhoneNum   string
}

func init() {
	users = make(map[string]*UserModel)
	tool.ReadGobFile(userDataFileName, &users)
	temp, _ := beego.GetConfig("String", "superUser", "")
	superUser = temp.(string)
}

//获取管理员电话
func GetManagerPhones() string {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	managers := strings.Split(superUser, "|")
	managerPhones := ""
	for _, name := range managers {
		if user, ok := users[name]; ok {
			if managerPhones != "" {
				managerPhones += ","
			}
			managerPhones += user.PhoneNum
		}
	}
	return managerPhones
}

//判断是否有权限
func JudgeIsHadPermission(useName string, commandType int) (bool, string) {
	//如果是超级管理员则一定有更新用户权限
	if strings.Contains(superUser, useName) && commandType == CommandType_UpdateUser {
		return true, ""
	}

	//获取权限
	userDataLock.Lock()
	defer userDataLock.Unlock()
	permission := 0
	if user, ok := users[useName]; ok {
		permission = user.Permission
	}

	//判断是否有操作权限
	if !tool.Tool_BitTest(permission, uint(commandType+1)) {
		return false, "没有指令权限，请联系管理员！"
	}

	return true, ""
}

//获取用户电话
func GetUserPhone(_senderNick string) (phoneNum string) {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	if user, ok := users[_senderNick]; ok {
		phoneNum = user.PhoneNum
	}
	return
}

//获取所有用户信息
func GetAllUserInfo() string {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	if len(users) <= 0 {
		return "当前没有任何用户，请添加！"
	}
	result := "\n***********************以下是已有的用户配置***********************\n"
	for k, v := range users {
		permissions := ""
		for i := 0; i < int(CommandType_Max); i++ {
			if tool.Tool_BitTest(v.Permission, uint(i+1)) {
				permissions += GetCommandNameByType(i) + "|"
			}
		}
		result += fmt.Sprintf("%s,拥有指令权限：%s\n", k, permissions)
	}
	return result
}

//更新用户数据
func UpdateUserInfo(userInfo string) (result string) {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	userArr := strings.Split(userInfo, ";")
	for _, user := range userArr {
		if user == "" {
			continue
		}
		userInfos := strings.Split(user, ",")
		if len(userInfos) < 3 {
			result = "输入信息不合法，名字电话权限项目权限以英文逗号分割，如张三,158xxx,0"
			return
		}
		name := userInfos[0]
		phone := userInfos[1]
		permission := userInfos[2]
		if name == "" {
			result = "名字不能为空，名字电话权限项目权限以英文逗号分割，如张三,158xxx,0,xx项目"
			return
		}
		if phone == "" && permission == "" {
			//只有名字则删除用户
			delete(users, name)
			continue
		}
		user := new(UserModel)
		if _, ok := users[name]; ok {
			user = users[name]
		}
		if phone != "" {
			user.PhoneNum = phone
		}
		if permission != "" {
			//处理权限,用|分割拥有权限的枚举
			permissions := strings.Split(permission, "|")
			for _, v := range permissions {
				nPermission, _ := strconv.Atoi(v)
				if nPermission >= 0 {
					user.Permission = tool.Tool_BitSet(user.Permission, uint(nPermission+1))
				} else {
					//如果是负数则删除对应权限
					user.Permission = tool.Tool_BitClear(user.Permission, uint(math.Abs(float64(nPermission))+1))
				}
			}
		}
		users[name] = user
	}

	//编码并存储
	tool.SaveGobFile(userDataFileName, users)
	result = "更新用户数据成功"
	return
}
