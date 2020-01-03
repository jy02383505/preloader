package controllers

import (
	"context"
	"time"

	"github.com/astaxie/beego"
	"go.mongodb.org/mongo-driver/bson"
	"github.com/vmihailenco/msgpack/v4"

	"api/models"
)

// Operations about preload
type ReportController struct {
	beego.Controller
}

func (this *ReportController) Report() {

	var tp models.ReportPackage
	var err error

	if err = msgpack.Unmarshal(this.Ctx.Input.RequestBody, &tp); err == nil {

		Rid := tp.Rid
		Uid := tp.Uid
		Sid := tp.Sid
		CheckValue := tp.CheckValue
		Status := tp.Status

		StreamName := Uid + "Devices"
		GroupName := Uid + "Group"

		beego.Debug(Rid, Uid, Sid)

		Devices, err := models.RedisClient.XRange(StreamName, Sid, Sid).Result()
		if err != nil {
			beego.Error("get XRange fail！", err, StreamName, Sid)
		}

		if len(Devices) > 0 {
			DeviceInfo := Devices[0]
			Preset := DeviceInfo.Values["Preset"].(string)
			Host := DeviceInfo.Values["Host"].(string)

			// 文件校对
			FileCheckResult := false
			if CheckValue == Preset{
				FileCheckResult = true
			}

			Filter := bson.D{{"rid", Rid}, {"uid", Uid}, {"host", Host}}

			var TaskStatus string
			if Status == "0" {
				TaskStatus = models.FailDStr
			} else {
				TaskStatus = models.SuccessStr
			}

			UpdateValue := bson.D{
				{"checkValue", tp.CheckValue},
				{"status", TaskStatus},
				{"finishTime", time.Now()},
				{"Rate", tp.Rate},
				{"checkResult", FileCheckResult}}

			Update := bson.D{{"$set", UpdateValue}}

			var Task bson.M
			updateErr := models.TaskConn.FindOneAndUpdate(context.Background(), Filter, Update).Decode(&Task)

			if updateErr != nil {
				beego.Error("FindOneAndUpdate", updateErr)
			}

			XAckRes, err := models.RedisClient.XAck(StreamName, GroupName, Sid).Result()
			if err != nil {
				beego.Error("redis XAck fail!", err, StreamName, GroupName, Sid, XAckRes)
			}

			XDelRes, err := models.RedisClient.XDel(StreamName, Sid).Result()
			if err != nil {
				beego.Error("redis XDel fail!", err, StreamName, Sid, XDelRes)
			}

			XLenRes, err := models.RedisClient.XLen(StreamName).Result()
			if err != nil {
				beego.Error("redis XLen fail!", err, StreamName, XLenRes)
			}

			if XLenRes > 0 {
				// 继续下发新的任务
				SendEdgeTask := models.SendEdgeTask{
					Rid: Rid,
					Uid: Uid,
					GroupName: GroupName,
				}

				TaskSendChannel <- SendEdgeTask
			} else {
				// 完成所有任务删除队列
				_, err := models.RedisClient.Del(StreamName).Result()
				if err != nil {
					beego.Error("redis Del fail!", err, StreamName)
				}

				// 更改改uid任务状态
				SuccessStatus := models.SuccessStr
				RIdSetRes, err := models.RedisClient.HSet(Rid, Uid, SuccessStatus).Result()
				if err != nil {
					beego.Error("redis hset fail!", Rid, Uid, RIdSetRes)
				}

				// 删除count计数
				SendCountKey := Uid + "Count"
				CountKey, err := models.RedisClient.Del(SendCountKey).Result()
				if err != nil {
					beego.Error("redis Del fail!", err, CountKey)
				}

				RidTask, err := models.RedisClient.Del(Rid).Result()
				if err != nil {
					beego.Error("redis Del fail!", err, RidTask)
				}
			}

			// 组织返回信息
			this.Data["json"] = map[string]interface{}{
				"result": "ok",
			}
		}else{
			beego.Error("get Devices is empty！", StreamName, Sid)

			// 组织返回信息
			this.Data["json"] = map[string]interface{}{
				"result": "fail",
			}
		}

	} else {
		this.Data["json"] = err.Error()
	}

	this.ServeJSON()

}
