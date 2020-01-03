package controllers

import (
	"time"
	"encoding/json"

	"github.com/astaxie/beego"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"api/models"
	"api/common"
)

// Operations about preload
type ReceiveController struct {
	beego.Controller
}


// @Title receive
// @Description 	接收数据
// @Param	body		body 	models.TaskPackage	true	{"username":"365noc","tasks": [{"url": "http://yqad.365noc.com/FTP/FTP_D/yx-8-liyangpingling/babaiban20181015/babaiban20181015_3D/0b1d5cb0-1782-42a8-bf08-ccd2e66d40f9_cpl.xml","validationType": "md5",  "validation": "9bd5e831444e2443fa37b231b01556a0"}],"runTime": "2018-10-16 17:25:00",  "bandwidth": "100k","compressed": 1,"layer":1 ,"header":"Referer: https://developer.mozilla.org/en-US/docs/Web/JavaScript"	}
// @Success 200 {"receive": "ok" }
// @Failure 403 body is empty
// @Failure 400 参数错误
// @router /receive [post]
func (this *ReceiveController) Receive() {

	var tp models.TaskPackage
	var err error
	if err = json.Unmarshal(this.Ctx.Input.RequestBody, &tp); err == nil {

		// 生成任务返回id
		Rid := primitive.NewObjectID().Hex()

		// 保存任务信息到mongo
		PreloadTask := models.PreloadTask {
			Rid: Rid,
			UserName: tp.UserName,
			Tasks : tp.Tasks,
			RunTime: tp.RunTime,
			BandWidth: tp.BandWidth,
			Compressed: tp.Compressed,
			Layer: tp.Layer,
			Header: tp.Header,
			CreateTime: time.Now(),
			ReadTimeout: tp.ReadTimeout,
		}
		TaskMongoChannel <- PreloadTask

		RecNum := len(tp.Tasks)    // 接收的任务数
		InvNum := 0    // 无效的任务数
		var ProcessingIds []string    // 任务详细id切片

		var TaskPkgList []models.TaskPkg    // task切片

		var ChannelNameSlice []string    // 频道名称切片

		for _, n := range tp.Tasks{

			Uid := primitive.NewObjectID().Hex()
			ChannelName := common.GetChannelByUrl(n.Url)
			if ChannelName != ""{
				TaskPkg := models.TaskPkg{
					Task: n,
					ChannelName: ChannelName,
					Uid: Uid,
				}
				ProcessingIds = append(ProcessingIds, Uid)

				TaskPkgList = append(TaskPkgList, TaskPkg)

				ChannelNameSlice = append(ChannelNameSlice, ChannelName)
			}else{
				InvNum++
				ProcessingIds = append(ProcessingIds, "null")
			}
		}

		TasksPkg := models.TasksPkg{
			Rid: Rid,
			TaskPkgList: TaskPkgList,
			UserName: tp.UserName,
			RunTime: tp.RunTime,
			BandWidth: tp.BandWidth,
			Compressed: tp.Compressed,
			Layer: tp.Layer,
			Header: tp.Header,
			ReadTimeout: tp.ReadTimeout,
			ChannelNameSlice: ChannelNameSlice,
		}
		TaskRedisChannel <- TasksPkg

		// 组织返回信息
		this.Data["json"] = map[string]interface{}{
			"status": "200",
			"r_id": Rid,
			"rec_num": RecNum,
			"inv_num": InvNum,
			"processing": ProcessingIds,
		}
	} else {
		this.Data["json"] = err.Error()
	}

	this.ServeJSON()
}

