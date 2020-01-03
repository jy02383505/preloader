package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type QueryPackage struct {
	Rid string `json:"rid"`
}

type PreloadTaskMongo struct {
	Id_ primitive.ObjectID `bson:"_id"`
	Rid string `bson:"rid"`
	Username string `bson:"username"`
	RunTime string `bson:"runtime"`
	Tasks []interface{} `bson:"tasks"`
	Bandwidth int `bson:"bandwidth"`
	Compressed int `bson:"compressed"`
	Layer int `bson:"layer"`
	Header string `bson:"header"`
	Status string `bson:"status"`
	CreateTime time.Time `bson:"createtime"`
	FinishedTime time.Time `bson:"finishedtime"`
}

type TaskMongo struct {
	Id_ primitive.ObjectID `bson:"_id"`
	RId string `bson:"rid"`
	Uid string `bson:"uid"`
	Host string `bson:"host"`
	Name string `bson:"name"`
	CreateTime time.Time `bson:"createtime"`
	Url string `bson:"url"`
	SendCode int `bson:"sendcode"`
	Status string `bson:"status"`
	FinishTime time.Time `bson:"finishtime"`
	CheckType string `bson:"checktype"`
	CheckValue string `bson:"checkValue"`
	Preset string `bson:"preset"`
	ReadTimeout int `bson:"readtimeout"`
	Compressed int `bson:"compressed"`
	Header string `bson:"header"`

}

type TaskCheck struct {
	Msg string `msgpack:"message"`
}