package controllers

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego"
	"github.com/go-redis/redis/v7"
	"go.mongodb.org/mongo-driver/bson"
	"github.com/vmihailenco/msgpack/v4"

	"api/models"
	"api/common"
)

var TaskChannelLen = 100
var TaskMongoChannel = make(chan models.PreloadTask, TaskChannelLen)    // mongo数据保存
var TaskRedisChannel = make(chan models.TasksPkg, TaskChannelLen)    // redis数据保存
var TaskSendChannel = make(chan models.SendEdgeTask, TaskChannelLen)    // 边缘任务下发



// 查询结果定时器
func QueryTask() {
	for {

		TaskFilter := bson.D{{"status", models.ProcessingStr}}
		cursor, err := models.TaskConn.Find(context.Background(), TaskFilter)
		if err != nil {
			beego.Error("Find fail", TaskFilter)
			panic(err)
		}

		var AllTask []models.TaskMongo
		if err = cursor.All(context.TODO(), &AllTask); err != nil {
			beego.Error("Find fail", err)
			panic(err)
		}

		for _, Task := range AllTask {

			SubTime := int(time.Now().Sub(Task.CreateTime).Seconds())

			EdgeTaskCheckUrl := beego.AppConfig.String("EdgeTaskCheckUrl")
			EdgeTaskCheckUrl = strings.Replace(EdgeTaskCheckUrl, "{host}", Task.Host, -1)

			if SubTime > models.TaskTimeOut{
				SendEdgeCheckInfo := models.SendEdgeCheckInfo{
					Uid: Task.Uid,
				}
				SendEdgeCheckArgs, err := msgpack.Marshal(&SendEdgeCheckInfo)
				if err != nil {
					beego.Error("Maspack SendEdgeInfo error!", SendEdgeCheckArgs)
				}

				// TODO 下发任务 url args
				beego.Debug("Send edge check task", EdgeTaskCheckUrl, string(SendEdgeCheckArgs))

				StatusCode, _, err := common.HttpPost(EdgeTaskCheckUrl, string(SendEdgeCheckArgs))

				beego.Debug(StatusCode)

				if err != nil || StatusCode == 404{
					beego.Error("Send edge task task error! ")
					var MTask bson.M
					Filter := bson.D{{"uid", Task.Uid}, {"host", Task.Host}}
					Update := bson.D{{"$set", bson.D{{"status", models.FailDStr}}}}
					updateErr := models.TaskConn.FindOneAndUpdate(context.Background(), Filter, Update).Decode(&MTask)

					if updateErr != nil {
						beego.Debug("FindOneAndUpdate", updateErr)
						panic(updateErr)
					}
					return
				}
			}
		}

		ticker := time.NewTicker(time.Second * 1)
		<-ticker.C
	}

}

// 保存任务信息到mongo
func SaveToMongo() {
	for {
		select {
		case task := <-TaskMongoChannel:

			_, err := models.TaskPkgConn.InsertOne(context.Background(), task)
			if err != nil {
				beego.Error("insert mongo error !", err, task)
			}
		}
	}
}


// 向redis保存任务状态rid,uid hash,保存uid与devices stream
func SendToRedis() {
	for {
		select {
		case task := <- TaskRedisChannel:

			Rid := task.Rid

			// 通信cms获取回源数据
			ChannelSrcInfo := SyncChannelSrc(task.ChannelNameSlice)

			for _, n := range task.TaskPkgList{

				Uid := n.Uid
				ChannelName := n.ChannelName
				Url := n.Task.Url

				// 将rid与uid的hash关系保存到redis
				RIdSetRes, err := models.RedisClient.HSet(Rid, Uid, 0).Result()
				if err != nil {
					beego.Error("redis HSet fail!", err, Rid, Uid, RIdSetRes)
					break
				}

				// 通过比对文件大小与传参限制计算下发设备数 并保存到redis里
				SendCount := SendDeviceCount(Url, task.BandWidth, ChannelSrcInfo[ChannelName])
				SendCountKey := Uid + "Count"
				err = models.RedisClient.Set(SendCountKey, SendCount, 0).Err()
				if err != nil {
					beego.Error("redis count set fail!", err, Uid, SendCount)
					break
				}

				// 获取设备列表 并将设备列表与uid关系保存到redis stream中
				StreamName := Uid + "Devices"

				ChannelDevices := GetChannelDevices(ChannelName, task.Layer)
				for _, d := range ChannelDevices{
					TaskMap := map[string] interface{} {
						"Host": d.Host,
						"Name": d.Name,
						"Url": Url,
						"CheckType": n.Task.CheckType,
						"Preset": n.Task.Preset,
						"Compressed": task.Compressed,
						"Header": task.Header,
						"ReadTimeout": task.ReadTimeout,
					}

					AddArgs := redis.XAddArgs{
						Stream: StreamName,
						Values: TaskMap,
						ID:     "*",
					}

					XAddRes, err := models.RedisClient.XAdd(&AddArgs).Result()

					if err != nil {
						beego.Error("redis stream xadd fail！", err, StreamName, XAddRes)
						break
					}

					// 在mongo保存边缘任务数据
					Task := models.EdgeTask{
						Rid: Rid,
						Uid: Uid,
						Host: d.Host,
						Name: d.Name,
						CreateTime: time.Now(),
						Url: Url,
						Status: models.ProcessingStr,
						CheckType: n.Task.CheckType,
						Preset: n.Task.Preset,
						ReadTimeout: task.ReadTimeout,
						Compressed: task.Compressed,
						Header: task.Header,
					}

					insertOneResult, err := models.TaskConn.InsertOne(context.Background(), Task)
					if err != nil {
						beego.Error("insert mongo error !", err, Uid, d.Host, insertOneResult)
					}
				}

				SendEdgeTask := models.SendEdgeTask {
					Rid: Rid,
					Uid: Uid,
					GroupName: "",
				}

				TaskSendChannel <- SendEdgeTask
			}
		}
	}
}


// 向边缘下发任务
func SendTaskToEdge() {
	for {
		select {
		case task := <-TaskSendChannel:

			Uid := task.Uid
			Rid := task.Rid
			GroupName := task.GroupName

			SendCountKey := Uid + "Count"
			SendCountString, err := models.RedisClient.Get(SendCountKey).Result()
			if err != nil {
				beego.Error("get send count fail！", SendCountKey)
			}

			SendCount, err := strconv.Atoi(SendCountString)
			if err != nil {
				beego.Error("get send count fail！", SendCountString)
			}

			if SendCount > 0 {

				StreamName := Uid + "Devices"

				Devices, err := models.RedisClient.XRange(StreamName, "-", "+").Result()
				if err != nil {
					beego.Error("get XRange fail！", err, StreamName)
				}
				for i := 0; i < SendCount; i++ {

					DeviceInfo := Devices[i]

					Host := DeviceInfo.Values["Host"].(string)
					Name := DeviceInfo.Values["Name"].(string)
					Url := DeviceInfo.Values["Url"].(string)
					CheckType := DeviceInfo.Values["CheckType"].(string)
					Compressed := DeviceInfo.Values["Compressed"].(string)
					Header := DeviceInfo.Values["Header"].(string)
					ReadTimeout := DeviceInfo.Values["ReadTimeout"].(string)

					Sid := DeviceInfo.ID

					if GroupName == ""{
						// 创建消费组
						GroupName = Uid + "Group"

						_, err := models.RedisClient.XGroupCreate(StreamName, GroupName, "0-0").Result()
						if err != nil {
							beego.Error("get XGroupCreate fail！", err, StreamName, GroupName)
						}
					}

					Streams := []string {StreamName, ">"}

					// 创建消费者
					ConsumerName := Name
					XReadGroupArgs := redis.XReadGroupArgs{
						Group: GroupName,
						Consumer: ConsumerName,
						Streams: Streams,
						Count: 1,

					}
					ReadRes, err := models.RedisClient.XReadGroup(&XReadGroupArgs).Result()
					if err != nil {
						beego.Error("get XReadGroup fail！", err, GroupName, ConsumerName)
					}

					EdgeReportUrl := beego.AppConfig.String("EdgeReportUrl")
					EdgeReportUrl = strings.Replace(EdgeReportUrl, "{host}", Host, -1)

					// 组装下发边缘参数
					CompressedInt, err := strconv.Atoi(Compressed)
					if err != nil {
						panic(err)
					}

					// 组装下发边缘参数
					ReadTimeoutInt, err := strconv.Atoi(ReadTimeout)
					if err != nil {
						panic(err)
					}

					SendEdgeInfo := models.SendEdgeInfo{
						Rid: Rid,
						Uid: Uid,
						Sid: Sid,
						LvsAddress: Host,
						Url: Url,
						CheckType: CheckType,
						HeaderString: Header,
						ReadTimeout: ReadTimeoutInt,
						ReportAddress: beego.AppConfig.String("ReportAddress"),
						Compressed: CompressedInt,
					}
					SendEdgeArgs, err := msgpack.Marshal(&SendEdgeInfo)
					if err != nil {
						beego.Error("Maspack SendEdgeInfo error! ", ReadRes)
					}
					// TODO 下发任务 url args
					beego.Debug("Send edge task", EdgeReportUrl, string(SendEdgeArgs))

					//StatusCode := 200
					StatusCode, SendRes, err := common.HttpPost(EdgeReportUrl, string(SendEdgeArgs))

					if err != nil {
						beego.Error("Send edge task error! ", ReadRes)
						//panic(err)
					}
					beego.Debug(StatusCode, string(SendRes))

					Filter := bson.D{{"rid", Rid}, {"uid", Uid}, {"host", Host}}
					UpdateValue := bson.D{{"sendcode", StatusCode}, {"createtime", time.Now()}}
					Update := bson.D{{"$set", UpdateValue}}

					var Task bson.M
					updateErr := models.TaskConn.FindOneAndUpdate(context.Background(), Filter, Update).Decode(&Task)

					if updateErr != nil {
						beego.Error("FindOneAndUpdate", updateErr)
					}
				}
			}
		}
	}

}


// 通信cms接口获取频道的回源信息
func SyncChannelSrc(ChannelName []string) map[string]models.ChannelSrcInfo {

	url := beego.AppConfig.String("CmsSrcUrl")

	args := map[string]interface{} {
		"ROOT": map[string]interface{} {
			"BODY": map[string]interface{} {
				"BUSI_INFO": map[string]interface{} {
					"chnNames": ChannelName,
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

	SrcInfo := map[string]models.ChannelSrcInfo {}

	ArgsJson, _ := json.Marshal(args)
	mString := string(ArgsJson)

	_, CmsReqBody, err := common.HttpPost(url, mString)

	if err != nil {
		return SrcInfo
	}

	var csr models.CmsSrcReq
	var JsonErr error

	if JsonErr = json.Unmarshal(CmsReqBody, &csr); JsonErr != nil {
		return SrcInfo
	}

	for _, n := range csr.Root.BODY.OutData.ChnBackInfo {

		ChannelDetail := n.BackDetail[0]

		csi := models.ChannelSrcInfo {
			SrcType : n.BackType,
			SrcIps : ChannelDetail.BackIps,
			SrcDum : ChannelDetail.BackSrcDmn,
		}

		SrcInfo[n.ChnName] = csi
	}

	return SrcInfo
}


// 获取预加载文件大小 单位字节（1byte=8bits) 返回单位Kbps
func GetFileLength(Url string) float64 {

	var err error

	ReqHead, err := common.HttpHead(Url)

	if err != nil {
		return 0
	}

	ContentLength := ReqHead.ContentLength()

	FileLength := float64(ContentLength) / 1000 * 8 / 300

	return FileLength
}


// 通信cms接口获取频道的设备列表
func GetChannelDevices(ChannelName string, Layer int) []models.DeviceInfo {
	beego.Debug(ChannelName)
	// TODO 接口暂时只支持channel_id
	ChannelCode := "99343"
	ChannelName = ChannelCode

	var ChannelDevices []models.DeviceInfo

	DevMap := make(map[int][]models.DeviceInfo)

	switch Layer {

	case 1:    // 只获取上层设备
		FirstLayerUrl := beego.AppConfig.String("CmsDeviceFirstLayerUrl")
		FirstLayerUrl = strings.Replace(FirstLayerUrl, "{channel}", ChannelName, -1)

		var fdr models.DeviceReq
		var JsonErr error
		FirstDeviceReqBody, err := common.HttpGet(FirstLayerUrl)

		if err != nil {
			return ChannelDevices
		}

		if JsonErr = json.Unmarshal(FirstDeviceReqBody, &fdr); JsonErr != nil {
			return ChannelDevices
		}

		for _, n := range fdr.Devices {
			LayerNum := n.LayerNum

			DeviceInfo := models.DeviceInfo {
				Host: n.Host,
				Name: n.Name,
			}
			DevMap[LayerNum] = append(DevMap[LayerNum], DeviceInfo)
		}
	case 2:    // 只获取下层设备
		LayerUrl := beego.AppConfig.String("CmsDeviceUrl")
		LayerUrl = strings.Replace(LayerUrl, "{channel}", ChannelName, -1)

		var fdr models.DeviceReq
		var JsonErr error

		DeviceReqBody, err := common.HttpGet(LayerUrl)

		if err != nil {
			return ChannelDevices
		}

		if JsonErr = json.Unmarshal(DeviceReqBody, &fdr); JsonErr != nil {
			return ChannelDevices
		}

		for _, n := range fdr.Devices {
			LayerNum := n.LayerNum
			DeviceInfo := models.DeviceInfo {
				Host: n.Host,
				Name: n.Name,
			}
			DevMap[LayerNum] = append(DevMap[LayerNum], DeviceInfo)
		}

	default:    // 同时获取上下层设备
		LayerUrl := beego.AppConfig.String("CmsDeviceUrl")
		LayerUrl = strings.Replace(LayerUrl, "{channel}", ChannelName, -1)

		var dr, fdr models.DeviceReq
		var JsonErr, JsonFirstErr error

		DeviceReqBody, err := common.HttpGet(LayerUrl)

		if err != nil {
			return ChannelDevices
		}

		if JsonErr = json.Unmarshal(DeviceReqBody, &dr); JsonErr != nil {
			return ChannelDevices
		}

		for _, n := range dr.Devices {
			LayerNum := n.LayerNum
			DeviceInfo := models.DeviceInfo {
				Host: n.Host,
				Name: n.Name,
			}
			DevMap[LayerNum] = append(DevMap[LayerNum], DeviceInfo)
		}

		FirstLayerUrl := beego.AppConfig.String("CmsDeviceFirstLayerUrl")
		FirstLayerUrl = strings.Replace(FirstLayerUrl, "{channel}", ChannelName, -1)

		FirstDeviceReqBody, err := common.HttpGet(FirstLayerUrl)

		if err != nil {
			return ChannelDevices
		}

		if JsonErr = json.Unmarshal(FirstDeviceReqBody, &fdr); JsonFirstErr != nil {
			return ChannelDevices
		}

		for _, n := range fdr.Devices {
			LayerNum := n.LayerNum
			DeviceInfo := models.DeviceInfo {
				Host: n.Host,
				Name: n.Name,
			}
			DevMap[LayerNum] = append(DevMap[LayerNum], DeviceInfo)
		}
	}

	keys := make([]int, 0, len(DevMap))
	for k := range DevMap {
		keys = append(keys, k)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, n := range keys {
		for _, DeviceInfo := range DevMap[n] {
			ChannelDevices = append(ChannelDevices, DeviceInfo)
		}
	}

	// TODO 暂时写死发到测试设备
	TempDeviceInfo := models.DeviceInfo {
		Host: "223.202.202.15",
		Name: "BGP-SM-4-3g9",
	}
	ChannelDevices = []models.DeviceInfo{TempDeviceInfo}

	return ChannelDevices
}


// 通过回源信息与参数限制计算下发设备数量
func SendDeviceCount(Url string, Bandwidth int, SrcInfo models.ChannelSrcInfo) int {

	BandwidthFloat := float64(Bandwidth)

	u, err := url.Parse(Url)

	if err != nil {
		return 0
	}

	var FileLen float64

	switch SrcInfo.SrcType {
	case "IPCOMMONCONFIG":
		for _, IP := range SrcInfo.SrcIps {

			var ChannelBuilder strings.Builder
			ChannelBuilder.WriteString(u.Scheme)
			ChannelBuilder.WriteString("://")
			ChannelBuilder.WriteString(IP)
			ChannelBuilder.WriteString(u.Path)
			SrcFilePath := ChannelBuilder.String()

			FileLen = GetFileLength(SrcFilePath)

			if FileLen != 0{
				break
			}
		}
	case "ORIGDOMAIN":
		var ChannelBuilder strings.Builder
		ChannelBuilder.WriteString(u.Scheme)
		ChannelBuilder.WriteString("://")
		ChannelBuilder.WriteString(SrcInfo.SrcDum)
		ChannelBuilder.WriteString(u.Path)
		SrcFilePath := ChannelBuilder.String()

		FileLen = GetFileLength(SrcFilePath)
	}

	SendCount := int(BandwidthFloat / FileLen)

	if SendCount < 1 {
		SendCount = 1
	}

	return SendCount
}