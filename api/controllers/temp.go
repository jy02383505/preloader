package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego"
	"api/common"

	//traceconfig "binTest/jaegerTest/CSJaeger/tracelib"
	//
	//"github.com/opentracing/opentracing-go"
	//"github.com/opentracing/opentracing-go/ext"
	//"github.com/opentracing/opentracing-go/log"

)


type TempController struct {
	beego.Controller
}


func (this *TempController) Temp() {

	ChannelName := "http://yqad.365noc.com"

	url := "http://223.202.75.137:32000/bm-app/apir/9110/channel/flexicache/firstlayer"

	args := map[string]interface{} {
		"ROOT": map[string]interface{} {
			"BODY": map[string]interface{} {
				"BUSI_INFO": map[string]interface{} {
					"channelName": ChannelName,
				},
			},
			"HEADER": map[string]interface{} {
				"AUTH_INFO": map[string]string {
					"FUNC_CODE": "9042",
					"LOGIN_NO": "novacdn",
					"LOGIN_PWD": "",
					"OP_NOTE": "",
				},
			},
		},
	}

	ArgsJson, _ := json.Marshal(args)
	mString := string(ArgsJson)

	_, CmsReqBody, _ := common.HttpPost(url, mString)



	fmt.Println(string(CmsReqBody))

}
