package models

import "time"

// 接受提交任务参数
type Task struct {
	Url string `json:"url"`
	CheckType string `json:"checkType"`
	Preset string `json:"preset"`
}

type TaskPackage struct {
	UserName string `json:"username"`
	Tasks []Task
	RunTime string `json:"runTime"`
	BandWidth int `json:"bandwidth"`    // 单位Kbps
	Compressed int    `json:"compressed"`
	Layer int    `json:"layer"`
	Header string `json:"header"`
	ReadTimeout int `json:"readTimeout"`
}

// redis save channel
type TasksPkg struct {
	Rid string
	TaskPkgList []TaskPkg
	UserName string
	RunTime string
	BandWidth int
	Compressed int
	Layer int
	Header string
	ChannelNameSlice []string
	ReadTimeout int
}

type TaskPkg struct {
	Task
	ChannelName string
	Uid string
}

// 用于保存到mongo
type PreloadTask struct {
	Rid string
	UserName string
	Tasks []Task
	RunTime string
	BandWidth int
	Compressed int
	Layer int
	Header string
	CreateTime time.Time
	ReadTimeout int
}


type SendEdgeTask struct {
	Rid string
	Uid string
	GroupName string
}

type CmsSrcReqBackDetailUnit struct {
	BackSrcDmn string `json:"backsrcDmn"`
	BackStrg string `json:"backStrg"`
	BackIps []string `json:"backIps"`
	BackUpIps []string `json:"backUpIps"`
}

type CmsSrcReqChnBackInfo struct {
	HostHeader string `json:"hostHeader"`
	BackDetail []CmsSrcReqBackDetailUnit
	ChnName string `json:"chnName"`
	BackType string `json:"backType"`
}

type CmsSrcReqOutData struct {
	ChnBackInfo []CmsSrcReqChnBackInfo    `json:"CHN_BACK_INFO"`
}

type CmsSrcReqBody struct {
	OutData CmsSrcReqOutData    `json:"OUT_DATA"`
	ReturnCode string `json:"RETURN_CODE"`
	ReturnMsg string `json:"RETURN_MSG"`
}

type CmsSrcReqRoot struct {
	HEADER interface{}    `json:"HEADER"`
	BODY CmsSrcReqBody `json:"BODY"`
}

type CmsSrcReq struct {
	Root CmsSrcReqRoot `json:"ROOT"`
}


// 回源信息
type ChannelSrcInfo struct {
	SrcType string
	SrcIps []string
	SrcDum string
}

type DeviceReqDevices struct {
	FirstLayer bool `json:"firstLayer"`
	Host string `json:"host"`
	LayerNum int `json:"layerNum"`
	Name string `json:"name"`
	Port int `json:"port"`
	SerialNumber string `json:"serialNumber"`
	Status string `json:"status"`
}

type DeviceReq struct {
	Code string `json:"code"`
	Devices []DeviceReqDevices    `json:"devices"`
	isLayered bool `json:"isLayered"`
}

type DeviceInfo struct {
	Host string
	Name string
}

// 下发边缘信息
type SendEdgeInfo struct {
	Rid string `msgpack:"rid"`
	Uid string `msgpack:"uid"`
	Sid string `msgpack:"sid"`
	LvsAddress string `msgpack:"lvs_address"`
	Url string `msgpack:"url"`
	CheckType string `msgpack:"check_type"`
	HeaderString string `msgpack:"header_string"`
	ReportAddress string `msgpack:"report_address"`
	Compressed int `msgpack:"compressed"`
	ReadTimeout int `msgpack:"read_timeout"`
}

// 边缘任务保存
type EdgeTask struct {
	Rid string
	Uid string
	Host string
	Name string
	CreateTime time.Time
	Url string
	SendCode int
	Status string
	CheckType string
	Preset string
	ReadTimeout int
	Compressed int
	Header string
}

// 下发边缘检查信息
type SendEdgeCheckInfo struct {
	Uid string `msgpack:"uid"`
}