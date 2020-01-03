package controllers

import (
	"context"
	"encoding/json"

	"github.com/astaxie/beego"
	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/bson"

	"api/models"
)

// Operations about preload
type QueryController struct {
	beego.Controller
}

func (this *QueryController) Query() {

	var tp models.QueryPackage
	var err error

	if err = json.Unmarshal(this.Ctx.Input.RequestBody, &tp); err == nil {

		Rid := tp.Rid

		PkgFilter := bson.D{{"rid", Rid}}

		var TaskPackage models.PreloadTaskMongo
		err = models.TaskPkgConn.FindOne(context.Background(), PkgFilter).Decode(&TaskPackage)

		if err != nil {
			beego.Error("FindOne fail", PkgFilter)
		}

		ReturnInfo := map[string]interface{}{}

		// 任务数量PreloadTaskMongo
		Total := len(TaskPackage.Tasks)

		TaskFilter := bson.D{{"rid", Rid}}
		cursor, err := models.TaskConn.Find(context.Background(), TaskFilter)
		if err != nil {
			beego.Error("Find fail", TaskFilter)
		}

		var AllTask []models.TaskMongo
		if err = cursor.All(context.TODO(), &AllTask); err != nil {
			beego.Error("Find fail", err)
		}


		TaskMap := map[string][]models.TaskMongo{}
		for _, Task := range AllTask {

			TaskMap[Task.Uid] = append(TaskMap[Task.Uid], Task)
		}

		AllTasks, err := models.RedisClient.HGetAll(Rid).Result()
		if err != nil {
			beego.Error("redis hgetAll fail!", Rid)
		}

		SuccessNum := 0

		ReturnTasks := map[string]string{}
		for Uid, status := range AllTasks {

			var StatusStr string
			switch status {
			case models.ProcessingStr:
				StatusStr = "PROCESSING"
			case models.SuccessStr:
				StatusStr = "SUCCESS"
				SuccessNum++
			case models.FailDStr:
				StatusStr = "FAILD"
			case models.CancelStr:
				StatusStr = "CANCEL"
			}

			ReturnTasks[Uid] = StatusStr
		}

		PkgTempNum := decimal.NewFromFloat(float64(SuccessNum)).Div(decimal.NewFromFloat(float64(Total)))
		PkgPercent := PkgTempNum.Mul(decimal.NewFromFloat(100))

		ReturnInfo["percent"] = PkgPercent    // 整体任务百分比

		var TaskList []map[string]interface{}

		for Uid, Tasks := range TaskMap {

			TaskMap := map[string]interface{}{}

			Status := ReturnTasks[Uid]

			if len(Tasks) <= 0{
				continue
			}

			TaskCnt := len(Tasks)

			FinishTime := Tasks[0].FinishTime.Format("2006-01-02 00:00:00")
			CreateTime := Tasks[0].CreateTime.Format("2006-01-02 00:00:00")
			var Url string
			var ValidationType interface{}
			var ValidationValue interface{}


			TaskSuccessNum := 0
			for _, task := range Tasks {
				if task.FinishTime.Format("2006-01-02 00:00:00") > FinishTime{
					FinishTime = task.FinishTime.Format("2006-01-02 00:00:00")
				}

				if task.CreateTime.Format("2006-01-02 00:00:00") < CreateTime{
					CreateTime = task.CreateTime.Format("2006-01-02 00:00:00")
				}

				Url = task.Url
				ValidationType = task.CheckType
				ValidationValue = task.CheckValue

				if task.Status == models.SuccessStr{
					TaskSuccessNum++
				}
			}

			if Status == "PROCESSING"{
				FinishTime = ""
			}

			TaskTemp := decimal.NewFromFloat(float64(TaskSuccessNum)).Div(decimal.NewFromFloat(float64(TaskCnt)))
			TaskPercent := TaskTemp.Mul(decimal.NewFromFloat(100))

			TaskMap["id"] = Uid
			TaskMap["url"] = Url
			TaskMap["status"] = Status
			TaskMap["percent"] = TaskPercent
			TaskMap["startTime"] = CreateTime
			TaskMap["finishTime"] = FinishTime
			TaskMap["validationType"] = ValidationType
			TaskMap["validationValue"] = ValidationValue

			TaskList = append(TaskList, TaskMap)
		}

		PreloadData := map[string]interface{}{
			"totalCount": Total,
			"tasks": TaskList,
		}

		ReturnInfo["rData"] = PreloadData
		this.Data["json"] = ReturnInfo
	} else {
		this.Data["json"] = err.Error()
	}

	this.ServeJSON()
}
