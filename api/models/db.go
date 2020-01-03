package models


import (
	"context"

	"github.com/astaxie/beego"
	"github.com/go-redis/redis/v7"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var RedisClient *redis.Client    // redis数据库
var MongoClient *mongo.Client    // mongo数据库数据库
var TaskTimeOut int    // 超时时间
var TaskPkgConn *mongo.Collection    // 预加载任务包Conn
var TaskConn *mongo.Collection    // 预加载各uid服务器Conn
var ProcessingStr string    // 正在进行任务标识
var SuccessStr string    // 成功任务标识
var FailDStr string    // 失败任务标识
var CancelStr string    // 中断任务标识



// 初始化接口数据库链接
func init() {

	TaskTimeOut, _ = beego.AppConfig.Int("TaskTimeOut")

	// 链接redis
	var RedisDBNUm, err = beego.AppConfig.Int("RedisDBNUm")
	if err != nil{
		beego.Error("AppConfig RedisDBNUm is error !")
		panic(err)
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr: beego.AppConfig.String("RedisAddress"),
		Password: beego.AppConfig.String("RedisPassword"),
		DB: RedisDBNUm,
	})

	// 链接mongo
	var MongoAddress = beego.AppConfig.String("MongoAddress")
	if MongoAddress == ""{
		beego.Error("AppConfig MongoAddress is error !")
		panic(err)
	}

	opts := options.Client().ApplyURI(MongoAddress)
	MongoClient, err = mongo.Connect(context.Background(), opts)

	if err != nil {
		beego.Error("Connects Mongo error !")
		panic(err)
	}

	if err = MongoClient.Ping(context.Background(), readpref.Primary()); err != nil {
		beego.Error("Mongo can not use !")
		panic(err)
	}

	TaskPkgConn = MongoClient.Database("PreLoader").Collection("TaskPackage")
	TaskConn = MongoClient.Database("PreLoader").Collection("Tasks")

	ProcessingStr = beego.AppConfig.String("PreloadProcessing")
	SuccessStr = beego.AppConfig.String("PreloadSuccess")
	FailDStr = beego.AppConfig.String("PreloadFailD")
	CancelStr = beego.AppConfig.String("PreloadCancel")

}
