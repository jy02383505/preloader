package cmd

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/vmihailenco/msgpack"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var HeadSend = "User-Agent:Mozilla/5.0 (Windows NT 6.3; WOW64 PRELOAD)"

// var TaskMap = map[string]*net.TCPConn{} // save the tcpconn for cancel the task

type dataAll struct {
	Rid           string `msgpack:"rid" json:"rid"`                 // "5dd6decc6857402b70b84667_rid"
	Uid           string `msgpack:"uid" json:"uid"`                 // "5dd6decc6857402b70b84667_uid"
	Sid           string `msgpack:"sid" json:"sid"`                 // "1574924938108-0_sid", #redis stream 对应id,为了方便做ack
	LvsAddress    string `msgpack:"lvs_address" json:"lvs_address"` // "61.164.154.194",#直拉请求地址获取文件"
	Url           string `msgpack:"url" json:"url"`
	CheckType     string `msgpack:"check_type" json:"check_type"` // 验证方式 MD5 或 BASIC
	HeaderString  string `msgpack:"header_string" json:"header_string"`
	ReportAddress string `msgpack:"report_address" json:"report_address"` // 汇报地址
	Compressed    int    `msgpack:"compressed" json:"compressed"`         // 1, 单压缩 2，单非压缩 3, 压缩和非压缩
	ReadTimeout   int    `msgpack:"read_timeout" json:"read_timeout"`     // 默认63秒，有值则使用该值作为边缘拉取资源的超时时间

	Rate       string `msgpack:"download_mean_rate" json:"download_mean_rate"`
	Status     string `msgpack:"status" json:"status"`
	CheckValue string `msgpack:"check_value" json:"check_value"`

	ContentLength int    `msgpack:"content_length" json:"content_length"`
	HttpStatus    int    `msgpack:"http_status" json:"http_status"`
	Msg           string `msgpack:"msg" json:"msg"`
}

func (data *dataAll) getKeyArr() *[2]string {
	arr := [2]string{data.Uid, fmt.Sprintf("%s_compressed", data.Uid)}
	return &arr
}

func (data dataAll) compressedIsThree() bool {
	if data.Compressed == 3 {
		return true
	}
	return false
}

type reportData struct {
	Rid           string `msgpack:"rid" json:"rid"`
	Uid           string `msgpack:"uid" json:"uid"`
	Sid           string `msgpack:"sid" json:"sid"`
	Status        string `msgpack:"status" json:"status"`
	Msg           string `msgpack:"msg" json:"msg"`
	CheckType     string `msgpack:"check_type" json:"check_type"`
	CheckValue    string `msgpack:"check_value" json:"check_value"`
	Rate          string `msgpack:"rate" json:"rate"`
	ContentLength int    `msgpack:"content_length" json:"content_length"`
}

type cancelData struct {
	Uid string `msgpack:"uid" json:"uid"`
}

func (data cancelData) getKeyArr() *[2]string {
	arr := [2]string{data.Uid, fmt.Sprintf("%s_compressed", data.Uid)}
	return &arr
}

type taskMap struct {
	// httpMap map[string]*net.TCPConn
	// httpsMap map[string]*tls.Conn
	tMap map[string]interface{}
}

// func (t taskMap) newTaskMap() {
// 	t.tMap = make(map[string]interface{})
// }

func (t taskMap) close(k string) {
	conn, ok := t.tMap[k]
	if ok {
		// conn.Close()
		switch c := conn.(type) {
		case *net.TCPConn:
			c.Close()
		case *tls.Conn:
			c.Close()
		}

	}
}

func task(c *gin.Context) {
	var data dataAll
	err := c.ShouldBindBodyWith(&data, binding.MsgPack)
	if err != nil {
		Log.Errorf("task[dataBindError.] err: %v", err)
	}
	Log.Debugf("task data: %+v", data)
	c.String(http.StatusOK, "ok")

	if data.compressedIsThree() {
		data.Compressed = 1
		go ProcessTask(data)
		data.Compressed = 2
		go ProcessTask(data)
	} else {
		ProcessTask(data)
	}
}

func RandInt(n int) int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	x := r.Intn(n)
	if x == 0 {
		return RandInt(n)
	}
	return x
}

// judge url http or https
func IsHttp(str string) bool {
	array_str := strings.SplitN(str, "/", 4)
	isHttp := true
	if len(array_str) > 0 {
		// Log.Debugf("IsHttp url: %s|| array_str[0]: %s", str, array_str[0])
		if array_str[0] == "https:" {
			isHttp = false
		}

	}
	return isHttp
}

func GetHost(str string) string {
	host := ""
	array_str := strings.SplitN(str, "/", 4)
	if len(array_str) < 3 {
		Log.Errorf("GetHost[parseStrError.] str: %v", str)
		host = "127.0.0.1"
	} else {
		host = array_str[2]
	}
	return host
}

func GetHeadLength(str string) (string, int, int, int) {
	var theContentLength int
	var err error
	var has_cl bool
	var has_cr bool
	req_arr := strings.SplitN(str, "\r\n\r\n", 2)
	thehead := req_arr[0]
	first_line := strings.SplitN(thehead, "\r\n", 2)[0]
	every_line_arr := strings.Split(thehead, "\r\n")
	for _, s_line := range every_line_arr {
		if strings.Contains(s_line, "Content-Length:") {
			has_cl = true
		}
		if strings.Contains(s_line, "Content-Range:") {
			has_cr = true
		}
		// Log.Debugf("GetHeadLength s_line: %v|| has_cl: %v|| has_cr: %v", s_line, has_cl, has_cr)
	}
	// 当Content-Range存在时，/后面的数字是全部内容的长度，这时的Content-Length也是存在的，但表示的是此次range请求要了多长。
	if has_cr {
		for _, s_line := range every_line_arr {
			if strings.Contains(s_line, "Content-Range:") {
				theContentLength, err = strconv.Atoi(strings.Replace(strings.SplitN(s_line, "/", 2)[1], " ", "", -1))
				if err != nil {
					Log.Debugf("GetHeadLength[theContentRength convert to int err]: %s|| s_line: %s", err, s_line)
				}
			}
		}
	} else if !has_cr && has_cl {
		for _, s_line := range every_line_arr {
			if strings.Contains(s_line, "Content-Length:") {
				theContentLength, err = strconv.Atoi(strings.Replace(strings.SplitN(s_line, ":", 2)[1], " ", "", -1))
				if err != nil {
					Log.Debugf("GetHeadLength theContentLength convert to int err: %s|| s_line: %s", err, s_line)
				}
			}
		}

	}
	http_status, err := strconv.Atoi(strings.SplitN(first_line, " ", 3)[1])
	if err != nil {
		Log.Debugf("GetHeadLength http_status convert to int error.http_status: %s", err)
	}
	head_length := len(thehead)
	return thehead, head_length, http_status, theContentLength
}

func getReadTimeout(data *dataAll) int {
	if data.ReadTimeout != 0 {
		return data.ReadTimeout
	}
	return readTimeout
}

func ProcessTask(data dataAll) {
	isHttp := IsHttp(data.Url)
	Log.Infof("ProcessTask Uid: %s|| url: %s|| HeadSend: %s|| isHttp: %t|| compressed: %v", data.Uid, data.Url, HeadSend, isHttp, data.Compressed)
	if isHttp == true {
		GetProxyHttp(ipTo, HeadSend, data)
	} else {
		GetProxyHttps(ipTo, HeadSend, data)
	}
	Log.Infof("ProcessTask[finished.] Uid: %s|| status: %s|| Url: %s", data.Uid, data.Status, data.Url)
}

func GetProxyHttp(ip_to string, head string, dataHttp dataAll) {
	// channel_name := getChannelName(dataHttp.Url)
	tcpAddr, err := net.ResolveTCPAddr("tcp", ip_to)
	if err != nil {
		Log.Debugf("GetProxyHttp[ResolveTCPAddrError: %v] Uid: %s|| url: %s", err, dataHttp.Uid, dataHttp.Url)
		return
	}

	tcpconn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		Log.Debugf("GetProxyHttp[DialTCPError: %v] Uid: %s|| url: %s", err, dataHttp.Uid, dataHttp.Url)
		dataHttp.Msg = fmt.Sprintf("%s", err)
		dataHttp.Status = "0"
		report(&dataHttp)
		return
	}

	// add Uid as key while tcpconn as value into taskMap
	k := dataHttp.Uid
	if dataHttp.Compressed == 2 {
		k = fmt.Sprintf("%s_compressed", dataHttp.Uid)
	}
	TaskMap.tMap[k] = tcpconn

	host := GetHost(dataHttp.Url)
	headerStr := dataHttp.HeaderString

	rawStr := ""
	if dataHttp.Compressed == 1 {
		rawStr = "GET " + dataHttp.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headerStr + "Accept-Encoding:gzip\r\n" + head + "\r\n\r\n"
	} else {
		rawStr = "GET " + dataHttp.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headerStr + head + "\r\n\r\n"
	}

	Log.Infof("GetProxyHttp headerStr: %v|| rawStr: %s|| Uid: %s|| url: %s", headerStr, rawStr, dataHttp.Uid, dataHttp.Url)
	//向tcpconn中写入数据
	_, err = tcpconn.Write([]byte(rawStr))
	if err != nil {
		Log.Errorf("GetProxyHttp[WriteError: %s] Uid: %s|| url: %s", err, dataHttp.Uid, dataHttp.Url)
		dataHttp.Msg = fmt.Sprintf("%s", err)
		dataHttp.Status = "0"
		report(&dataHttp)
		return

	}
	rTimeout := getReadTimeout(&dataHttp)
	buf := make([]byte, 8192)
	Log.Infof("GetProxyHttp rTimeout: %d|| Uid: %s|| url: %s", rTimeout, dataHttp.Uid, dataHttp.Url)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(rTimeout) * time.Second))

	head_end := false
	var (
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		totalLength             int
		head_length             int
		body_first_start        int
	)
	st := float64(time.Now().UnixNano() / 1e6)
	md5Ctx := md5.New()
	body_first := false
	for {
		length, errRead := tcpconn.Read(buf)
		if errRead != nil {
			tcpconn.Close()
			Log.Infof("GetProxyHttp[ReadError: %s] Uid: %s|| url: %s", errRead, dataHttp.Uid, dataHttp.Url)
			// get ContentLength and DownloadMeanRate
			dataHttp.ContentLength = totalLength - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if dataHttp.ContentLength < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(dataHttp.ContentLength) / 1024
				} else {
					d_rate = float64(dataHttp.ContentLength) / 1024 / float64(delta_t)
				}
			}
			dataHttp.Rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			Log.Debugf("GetProxyHttp totalLength: %d|| delta_t: %f|| Uid: %s|| url: %s", totalLength, delta_t, dataHttp.Uid, dataHttp.Url)
			if errRead == io.EOF {
				dataHttp.Status = "1"
				dataHttp.Msg = "Done"
			} else if strings.Contains(fmt.Sprintf("%s", errRead), "timeout") {
				dataHttp.Status = fmt.Sprintf("Timeout(%ds)", rTimeout)
				if dataHttp.ContentLength <= 0 {
					dataHttp.ContentLength = 0
				}
			} else if strings.Contains(fmt.Sprintf("%s", errRead), "use of closed network connection") {
				Log.Infof("GetProxyHttp[theConnectionCanceledByCenter.] uid: %v", dataHttp.Uid)
				break
			} else {
				dataHttp.Status = "0"
				dataHttp.Msg = fmt.Sprintf("%s", errRead)
			}
			report(&dataHttp)
			break
		}

		if length > 0 {
			totalLength += length
			tcpconn.SetDeadline(time.Now().Add(time.Duration(rTimeout) * time.Second))
		}

		if !head_end {
			recvStr := string(buf[:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = GetHeadLength(recvStr)
				o_content_length = original_content_length
				head_length += h_length
				head_end = true
				dataHttp.HttpStatus = http_status
				Log.Debugf("GetProxyHttp[headering.] o_content_length: %d|| head_length: %d|| http_status: %d|| thehead: %s|| Uid: %s|| url: %s", o_content_length, head_length, http_status, thehead, dataHttp.Uid, dataHttp.Url)
			}
		}

		// body begin.make the md5 checksum.
		if dataHttp.CheckType == "MD5" && totalLength >= head_length+4 {
			if !body_first {
				body_first = true
				body_first_start = head_length + 4
				Log.Debugf("GetProxyHttp[MD5.] body_first_start: %d|| Uid: %s|| url: %s", body_first_start, dataHttp.Uid, dataHttp.Url)
				if body_first_start != 0 {
					md5Ctx.Write(buf[body_first_start:length])
				}
			} else {
				md5Ctx.Write(buf[:length])
			}
		}

		// the length of receiving satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((totalLength - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			Log.Infof("GetProxyHttp[theLengthOfReceivingSatisfiedWithContent_Length,CloseConnectionInitiatively.] totalLength: %d|| head_length: %d|| o_content_length: %d|| Uid: %s|| url: %s", totalLength, head_length, o_content_length, dataHttp.Uid, dataHttp.Url)
			// get ContentLength and DownloadMeanRate
			dataHttp.ContentLength = totalLength - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if dataHttp.ContentLength < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(dataHttp.ContentLength) / 1024
				} else {
					d_rate = float64(dataHttp.ContentLength) / 1024 / float64(delta_t)
				}
			}
			dataHttp.Rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			Log.Infof("GetProxyHttp[finished.] totalLength: %d|| delta_t: %f|| Uid: %s|| url: %s", totalLength, delta_t, dataHttp.Uid, dataHttp.Url)
			dataHttp.Msg = "Done."
			dataHttp.Status = "1"
			if dataHttp.CheckType == "MD5" {
				// output md5 checksum
				cipherStr := md5Ctx.Sum(nil)
				dataHttp.CheckValue = hex.EncodeToString(cipherStr)
			}
			report(&dataHttp)
			break
		}
	}
}

func GetProxyHttps(ip_to string, head string, dataHttps dataAll) {
	// channel_name := getChannelName(dataHttps.Url)
	host := GetHost(dataHttps.Url)
	headerStr := dataHttps.HeaderString
	rawStr := ""
	if dataHttps.Compressed == 1 {
		rawStr = "GET " + dataHttps.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headerStr + "Accept-Encoding:gzip\r\n" + head + "\r\n\r\n"
	} else {
		rawStr = "GET " + dataHttps.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headerStr + head + "\r\n\r\n"
	}
	rTimeout := getReadTimeout(&dataHttps)
	Log.Debugf("GetProxyHttps rawStr: %s|| Uid: %s|| url: %s|| headerStr: %v|| rTimeout: %v", rawStr, dataHttps.Uid, dataHttps.Url, headerStr, rTimeout)
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	tcpconn, err := tls.Dial("tcp", sslIpTo, conf)
	if err != nil {
		Log.Errorf("GetProxyHttps err: %s|| Uid: %s|| url: %s", err, dataHttps.Uid, dataHttps.Url)
		dataHttps.Status = "0"
		dataHttps.Msg = fmt.Sprintf("%s", err)
		report(&dataHttps)
		return
	}

	// add Uid as key while tcpconn as value into taskMap
	k := dataHttps.Uid
	if dataHttps.Compressed == 2 {
		k = fmt.Sprintf("%s_compressed", dataHttps.Uid)
	}
	TaskMap.tMap[k] = tcpconn

	//向tcpconn中写入数据
	_, err = tcpconn.Write([]byte(rawStr))
	if err != nil {
		Log.Errorf("GetProxyHttps err: %s|| Uid: %s|| url: %s", err, dataHttps.Uid, dataHttps.Url)
		dataHttps.Status = "0"
		dataHttps.Msg = fmt.Sprintf("%s", err)
		report(&dataHttps)
		return
	}
	buf := make([]byte, 8192)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(rTimeout) * time.Second))

	head_end := false
	var (
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		totalLength             int
		head_length             int
		body_first              bool
		body_first_start        int
	)
	st := float64(time.Now().UnixNano() / 1e6)
	md5Ctx := md5.New()
	for {
		length, err := tcpconn.Read(buf)
		if err != nil {
			tcpconn.Close()
			Log.Infof("GetProxyHttps[ReadError: %v] Uid: %s|| url: %s", err, dataHttps.Uid, dataHttps.Url)
			dataHttps.ContentLength = totalLength - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if dataHttps.ContentLength < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(dataHttps.ContentLength) / 1024
				} else {
					d_rate = float64(dataHttps.ContentLength) / 1024 / float64(delta_t)
				}
			}
			dataHttps.Rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			Log.Infof("GetProxyHttps totalLength: %s|| delta_t: %s|| Uid: %s|| url: %s", totalLength, delta_t, dataHttps.Uid, dataHttps.Url)
			if err == io.EOF {
				dataHttps.Status = "1"
				dataHttps.Msg = "Done"
			} else if strings.Contains(fmt.Sprintf("%s", err), "timeout") {
				dataHttps.Status = "0"
				dataHttps.Msg = fmt.Sprintf("Timeout(%ds)", rTimeout)
				if dataHttps.ContentLength <= 0 {
					dataHttps.ContentLength = 0
				}
			} else if strings.Contains(fmt.Sprintf("%s", err), "use of closed network connection") {
				Log.Infof("GetProxyHttps[theConnectionCanceledByCenter.] uid: %v", dataHttps.Uid)
				break
			} else {
				dataHttps.Status = "0"
				dataHttps.Msg = fmt.Sprintf("%s", err)
			}
			report(&dataHttps)
			break
		}
		if length > 0 {
			totalLength += length
			tcpconn.SetDeadline(time.Now().Add(time.Duration(rTimeout) * time.Second))
		}
		if !head_end {
			recvStr := string(buf[0:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = GetHeadLength(recvStr)
				o_content_length = original_content_length
				dataHttps.HttpStatus = http_status
				head_length += h_length
				head_end = true
				Log.Infof("GetProxyHttps[headering.] o_content_length: %d|| head_length: %s|| http_status: %s|| thehead: %s|| Uid: %s|| url: %s", o_content_length, head_length, http_status, thehead, dataHttps.Uid, dataHttps.Url)
			}
		}

		// body begin.make the md5 checksum.
		if dataHttps.CheckType == "MD5" && totalLength >= head_length+4 {
			if !body_first {
				body_first = true
				body_first_start = head_length + 4
				Log.Infof("GetProxyHttps[MD5.] body_first_start: %d|| Uid: %s|| url: %s", body_first_start, dataHttps.Uid, dataHttps.Url)
				if body_first_start != 0 {
					md5Ctx.Write(buf[body_first_start:length])
				}
			} else {
				md5Ctx.Write(buf[:length])
			}
		}

		// the length of receiving satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((totalLength - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			Log.Infof("GetProxyHttps[theLengthOfReceivingSatisfiedWithContent_Length,closeConnectionInitiatively.] totalLength: %d|| head_length: %d|| o_content_length: %d|| Uid: %s|| url: %s", totalLength, head_length, o_content_length, dataHttps.Uid, dataHttps.Url)
			// get ContentLength and DownloadMeanRate
			dataHttps.ContentLength = totalLength - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if dataHttps.ContentLength < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(dataHttps.ContentLength) / 1024
				} else {
					d_rate = float64(dataHttps.ContentLength) / 1024 / float64(delta_t)
				}
			}
			dataHttps.Rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			Log.Infof("GetProxyHttps[finished.] totalLength: %d|| delta_t: %f|| Uid: %s|| url: %s", totalLength, delta_t, dataHttps.Uid, dataHttps.Url)
			dataHttps.Status = "1"
			dataHttps.Msg = "Done."
			if dataHttps.CheckType == "MD5" {
				// output md5 checksum
				cipherStr := md5Ctx.Sum(nil)
				dataHttps.CheckValue = hex.EncodeToString(cipherStr)
			}
			report(&dataHttps)
			break
		}
	}
}

func newReportData(data *dataAll) *reportData {
	var rData reportData
	rData.Uid = data.Uid
	rData.Rid = data.Rid
	rData.Sid = data.Sid
	rData.Status = data.Status
	rData.Msg = data.Msg
	rData.ContentLength = data.ContentLength
	rData.CheckType = data.CheckType
	if data.CheckType == "BASIC" {
		rData.CheckValue = fmt.Sprintf("%d", data.ContentLength)
	} else {
		rData.CheckValue = data.CheckValue
	}
	rData.Rate = data.Rate
	return &rData
}

func report(data *dataAll) {
	var (
		request  *http.Request
		response *http.Response
		r_status int
		rData    *reportData
	)
	rData = newReportData(data)

	for i := 0; i < 4; i++ {

		b, err := msgpack.Marshal(*rData)
		if err != nil {
			Log.Errorf("report json err:%s", err)
		}
		body := bytes.NewBuffer([]byte(b))
		Log.Debugf("report body: %+v|| reportData: %+v", body, *rData)
		reportAddress := data.ReportAddress + "?id=" + data.Uid

		//可以通过client中transport的Dial函数,在自定义Dial函数里面设置建立连接超时时长和发送接受数据超时
		client := &http.Client{
			Transport: &http.Transport{
				Dial: func(netw, addr string) (net.Conn, error) {
					conn, err := net.DialTimeout(netw, addr, time.Second*5) //设置建立连接超时
					if err != nil {
						return nil, err
					}
					conn.SetDeadline(time.Now().Add(time.Second * 8)) //设置发送接收数据超时
					return conn, nil
				},
				ResponseHeaderTimeout: time.Second * 3,
			},
		}
		if i > 0 {
			Log.Debugf("report body: %s|| Uid: %s|| url: %s", body, data.Uid, data.Url)
		}
		request, err = http.NewRequest("POST", reportAddress, body) //提交请求;用指定的方法，网址，可选的主体返回一个新的*Request
		if err != nil {
			Log.Errorf("report[NewRequestError: %s] Uid: %s|| url: %s", err, data.Uid, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		request.Header.Set("Content-Type", "application/msgpack")
		request.Header.Set("Host", "www.report.com")
		request.Header.Set("Accept", "*/*")
		request.Header.Set("User-Agent", "ChinaCache")
		request.Header.Set("X-CC-Preload-Report", "ChinaCache")
		request.Header.Set("Content-Length", fmt.Sprintf("%d", len(b)))
		request.Header.Set("Connection", "close")

		response, err = client.Do(request) //前面预处理一些参数，状态，Do执行发送；处理返回结果;Do:发送请求,
		if err != nil {
			Log.Debugf("report[retried No.%d times, err: %s] request: %+v, reportAddress: %s|| Uid: %s|| url: %s", i, err, request, reportAddress, data.Uid, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		r_status = response.StatusCode //获取返回状态码，正常是200
		if r_status != 200 {
			Log.Infof("report[retried No.%d times, r_status: %d] Uid: %s|| url: %s", i, r_status, data.Uid, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		defer response.Body.Close()
		r_body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			Log.Errorf("report[retried No.%d times, ReadAllError: %s] Uid: %s|| url: %s", i, err, data.Uid, data.Url)
			return
		}
		Log.Infof("report[retried No.%d times, r_status: %d] r_body: %s|| Uid: %s|| url: %s", i, r_status, r_body, data.Uid, data.Url)
		break
	}

	// clear the taskMap
	kArrPtr := data.getKeyArr()
	for _, theK := range *kArrPtr {
		delete(TaskMap.tMap, theK)
	}
	Log.Infof("report[deleteUid.] Uid: %s|| url: %s", data.Uid, data.Url)
}

func cancel(c *gin.Context) {
	var data cancelData
	err := c.ShouldBindBodyWith(&data, binding.MsgPack)
	if err != nil {
		Log.Errorf("cancel[dataBindError.] err: %v", err)
		return
	}
	kArr := data.getKeyArr()
	Log.Debugf("cancel[doneBefore.] data: %+v|| TaskMap.tMap(len: %v): %+v|| kArr: %v", data, len(TaskMap.tMap), TaskMap.tMap, kArr)

	// close the connection
	for _, theK := range kArr {
		TaskMap.close(theK)
		delete(TaskMap.tMap, theK)
	}

	Log.Debugf("cancel[done.] data: %+v|| httpMap(len: %v): %+v", data, len(TaskMap.tMap), TaskMap.tMap)
	c.String(http.StatusOK, "ok")
	return
}

func check(c *gin.Context) {
	var data cancelData
	err := c.ShouldBindBodyWith(&data, binding.MsgPack)
	if err != nil {
		Log.Errorf("check[dataBindError.] err: %v", err)
		return
	}
	rData := map[string]string{}
	msg := "Not found"
	rCode := 404
	kArr := data.getKeyArr()

	for _, theK := range kArr {
		_, ok := TaskMap.tMap[theK]
		if ok {
			rCode = 200
			msg = "processing"
			break
		}
	}

	rData["message"] = msg
	rBytes, _ := msgpack.Marshal(rData)
	Log.Debugf("check[done.] data: %+v|| TaskMap.tMap(len: %v): %+v|| rData: %+v", data, len(TaskMap.tMap), TaskMap.tMap, rData)
	c.Data(rCode, "application/msgpack", rBytes)
	return
}
