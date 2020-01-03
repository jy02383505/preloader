package controllers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/astaxie/beego"
	"go.mongodb.org/mongo-driver/bson"
	"github.com/vmihailenco/msgpack/v4"

	"api/models"
	"api/common"
)

// Operations about preload
type CancelController struct {
	beego.Controller
}

func (this *CancelController) Cancel() {
	var tp models.CancelPackage
	var err error

	ReturnInfo := map[string]string{}

	if err = json.Unmarshal(this.Ctx.Input.RequestBody, &tp); err == nil {
		Rid := tp.Rid
		Uid := tp.Uid

		var UidSlice []string

		if Rid != ""{

			AllTasks, err := models.RedisClient.HGetAll(Rid).Result()
			if err != nil {
				beego.Error("redis HGetAll fail!", err, Rid)
			}
			if len(AllTasks) > 0{
				for Uid, status := range AllTasks {
					if status == models.ProcessingStr {
						UidSlice = append(UidSlice, Uid)
					}
				}
			}
		}else{
			if Uid != ""{
				UidSlice = append(UidSlice, Uid)
			}
		}

		if len(UidSlice) > 0{
			for _, Uid := range UidSlice{

				SendEdgeCheckInfo := models.SendEdgeCheckInfo{
					Uid: Uid,
				}

				StreamName := Uid + "Devices"

				Devices, err := models.RedisClient.XRange(StreamName, "-", "+").Result()
				if err != nil {
					beego.Error("get XRange fail！", err, StreamName)
				}

				EdgeTaskCancelUrl := beego.AppConfig.String("EdgeTaskCancel")
				EdgeCancelArgs, err := msgpack.Marshal(&SendEdgeCheckInfo)
				if err != nil {
					beego.Error("Maspack SendEdgeInfo error!", EdgeCancelArgs)
				}

				beego.Debug(111111, Devices)

				// 对队列中设备下发中断操作
				for _, DeviceInfo := range Devices {
					Host := DeviceInfo.Values["Host"].(string)
					EdgeTaskCancelUrl = strings.Replace(EdgeTaskCancelUrl, "{host}", Host, -1)

					// TODO 下发任务 url args
					beego.Debug("Send edge cancel task", EdgeTaskCancelUrl, string(EdgeCancelArgs))

					_, _, err := common.HttpPost(EdgeTaskCancelUrl, string(EdgeCancelArgs))

					if err != nil{
						beego.Error("Send edge task cancel error!", EdgeTaskCancelUrl, Uid)
					}
				}

				// 删除队列
				DelRes, err := models.RedisClient.Del(StreamName).Result()
				if err != nil {
					beego.Error("Del stream result! ", err, DelRes)
				}

				// 删除count计数
				SendCountKey := Uid + "Count"
				CountKey, err := models.RedisClient.Del(SendCountKey).Result()
				if err != nil {
					beego.Error("redis Del fail!", err, CountKey)
				}

				var Task models.TaskMongo
				Filter := bson.D{{"uid", Uid}}
				err = models.TaskConn.FindOne(context.Background(), Filter).Decode(&Task)
				if err != nil {
					beego.Error("FindOne fail", err, Filter)
				}

				// 更改redis任务状态为中断
				RIdSetRes, err := models.RedisClient.HSet(Task.RId, Uid, models.CancelStr).Result()
				if err != nil {
					beego.Error("redis HSet fail!", err, Task.RId, Uid, RIdSetRes)
				}

				// 更改mongo任务状态为中断
				Update := bson.D{{"$set", bson.D{{"status", models.CancelStr}}}}
				_, updateErr := models.TaskConn.UpdateMany(context.Background(), Filter, Update)
				if updateErr != nil {
					beego.Error("UpdateMany", updateErr)
				}

				ReturnInfo["message"] = "ok"
			}

		}else{
			ReturnInfo["message"] = "Task ot found!"
		}
		this.Data["json"] = ReturnInfo
	} else {
		this.Data["json"] = err.Error()
	}

	this.ServeJSON()

}
