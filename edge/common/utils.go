package common

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

var Log *log.Logger

func String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func LogLevel(level string) log.Level {
	switch level {
	case "Debug":
		return log.DebugLevel
	case "Info":
		return log.InfoLevel
	case "Warn":
		return log.WarnLevel
	case "Error":
		return log.ErrorLevel
	case "Fatal":
		return log.FatalLevel
	case "Panic":
		return log.PanicLevel
	case "Trace":
		return log.TraceLevel
	default:
		return log.DebugLevel
	}
}

func init() {
	Log = log.New()
	// 设置显示文件及行号
	Log.SetReportCaller(true)
	Log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	Log.SetLevel(log.DebugLevel) //WarnLevel Debug,Info,Warn,Error,Fatal,Panic

}

func Base64Encode(src []byte) string {
	return base64.StdEncoding.EncodeToString(src)
}

func Base64Decode(src string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(src)
}

func randSeed() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

//Random get random in min between max. 生成指定大小的随机数.
func random(min int64, max int64) float64 {

	r := randSeed()
	if max <= min {
		panic(fmt.Sprintf("invalid range %d >= %d", max, min))
	}
	decimal := r.Float64()

	if max <= 0 {
		return (float64(r.Int63n((min*-1)-(max*-1))+(max*-1)) + decimal) * -1
	}
	if min < 0 && max > 0 {
		if r.Int()%2 == 0 {
			return float64(r.Int63n(max)) + decimal
		}
		return (float64(r.Int63n(min*-1)) + decimal) * -1
	}
	return float64(r.Int63n(max-min)+min) + decimal
}

//利用当前时间的UNIX时间戳初始化rand包
func Random() int {
	rand.Seed(time.Now().UnixNano())
	x := rand.Intn(60)
	return x
}
