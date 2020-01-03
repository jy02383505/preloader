package controllers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/astaxie/beego"
	"github.com/go-redis/redis/v7"
	"go.mongodb.org/mongo-driver/bson"

	"api/models"
	"api/common"
)

// Operations about preload
type RetryController struct {
	beego.Controller
}

func (this *RetryController) Retry() {

	var tp models.RetryPackage
	var err error

	if err = json.Unmarshal(this.Ctx.Input.RequestBody, &tp); err == nil {
		Uid := tp.Uid

		StreamName := Uid + "Devices"

		// 删除同key的任务
		DelRes, err := models.RedisClient.Del(StreamName).Result()
		if err != nil {
			beego.Error("Del stream result! ", err, DelRes)
		}

		Filter := bson.D{{"uid", Uid}, {"status", bson.D{{"$ne", models.SuccessStr}}}}
		cursor, err := models.TaskConn.Find(context.Background(), Filter)
		if err != nil {
			beego.Error("Find fail", Filter)
		}

		var AllTask []models.TaskMongo
		if err = cursor.All(context.TODO(), &AllTask); err != nil {
			beego.Error("Find fail", err, Filter)
		}

		var Rid string

		var ChannelNameSlice []string    // 频道名称切片
		for _, Task := range AllTask {
			Rid = Task.RId
			ChannelName := common.GetChannelByUrl(Task.Url)
			ChannelNameSlice = append(ChannelNameSlice, ChannelName)
		}

		ChannelSrcInfo := SyncChannelSrc(ChannelNameSlice)


		PkgFilter := bson.D{{"rid", Rid}}
		var TaskPackage models.PreloadTaskMongo
		err = models.TaskPkgConn.FindOne(context.Background(), PkgFilter).Decode(&TaskPackage)

		if err != nil {
			beego.Error("FindOne fail", PkgFilter)
		}

		for _, Task := range AllTask {

			ChannelName := common.GetChannelByUrl(Task.Url)

			// 通过比对文件大小与传参限制计算下发设备数 并保存到redis里
			SendCount := SendDeviceCount(Task.Url, TaskPackage.Bandwidth, ChannelSrcInfo[ChannelName])
			SendCountKey := Uid + "Count"
			err = models.RedisClient.Set(SendCountKey, SendCount, 0).Err()
			if err != nil {
				beego.Error("redis count set fail!", err, Uid, SendCount)
				break
			}

			TaskMap := map[string] interface{} {
				"Host": Task.Host,
				"Name": Task.Name,
				"Url": Task.Url,
				"CheckType": Task.CheckType,
				"Preset": Task.Preset,
				"ReadTimeout": Task.ReadTimeout,
				"Compressed": Task.Compressed,
				"Header": Task.Header,
			}

			AddArgs := redis.XAddArgs{
				Stream: StreamName,
				Values: TaskMap,
				ID:     "*",
			}

			XAddRes, err := models.RedisClient.XAdd(&AddArgs).Result()

			if err != nil {
				beego.Error("redis stream XAdd fail！", err, Uid, XAddRes)
			}


			Filter := bson.D{{"uid", Uid}, {"host", Task.Host}}

			UpdateValue := bson.D{
				{"status", models.ProcessingStr},
				{"createtime", time.Now()}}

			Update := bson.D{{"$set", UpdateValue}}

			var Task bson.M
			updateErr := models.TaskConn.FindOneAndUpdate(context.Background(), Filter, Update).Decode(&Task)

			if updateErr != nil {
				beego.Error("FindOneAndUpdate", updateErr)
			}
		}

		SendEdgeTask := models.SendEdgeTask {
			Rid: Rid,
			Uid: Uid,
			GroupName: "",
		}

		TaskSendChannel <- SendEdgeTask

	} else {
		this.Data["json"] = err.Error()
	}

	this.ServeJSON()


}

