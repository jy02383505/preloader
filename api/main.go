/*
 * @Author: huan.ma
 * @Date: 2019-11-21 16:51:44
 * @Last Modified by: huan.ma
 * @Last Modified time: 2019-11-22 14:36:16
 */
package main

import (
	"api/controllers"
	_ "api/routers"
	"log"
	"net/http"
	"os"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/grace"
)

func init() {
	beego.SetLogger("file", `{"filename":"logs/server.log"}`)
	beego.SetLevel(beego.LevelDebug)
	beego.SetLogFuncCall(true)
}

func main() {
	if beego.BConfig.RunMode == "dev" {
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	} else {
		mux := http.NewServeMux()
		err := grace.ListenAndServe("localhost:8080", mux)
		if err != nil {
			log.Println(err)
		}
		log.Println("Server on 8080 stopped")
		os.Exit(0)
	}
	go controllers.SaveToMongo()
	go controllers.SendToRedis()
	go controllers.SendTaskToEdge()
	go controllers.QueryTask()
	beego.Run()

}
