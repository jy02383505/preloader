package cmd

// version: 3.3

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var log = Log

// var TsOutputChan = make(chan map[string]interface{}, TsConcurrent_num)

type Task struct {
	Url              string              `json:"url"`
	Conn             int                 `json:"conn,omitempty"`
	Id               string              `json:"id,omitempty"`
	Rate             int                 `json:"rate,omitempty"`
	Check_type       string              `json:"check_type,omitempty"`
	Preload_address  string              `json:"preload_address,omitempty"`
	Priority         int                 `json:"priority,omitempty"`
	Nest_track_level int                 `json:"nest_track_level,omitempty"`
	Limit_rate       string              `json:"limit_rate,omitempty"`
	Md5              string              `json:"md5,omitempty"`
	Origin_Remote_IP string              `json:"origin_remote_ip,omitempty"`
	Header_list      []map[string]string `json:"header_list,omitempty"`
}

type DataFromCenter struct {
	Url_list            []Task `json:"url_list,omitempty"`
	Compressed_url_list []Task `json:"compressed_url_list,omitempty"`
	Is_override         int    `json:"is_override,omitempty"`
	Check_value         string `json:"check_value,omitempty"`
	Sessionid           string `json:"sessionid,omitempty"`
	Lvs_address         string `json:"lvs_address,omitempty"`
	DataType            string `json:"dataType,omitempty"`
	Action              string `json:"action,omitempty"`
	Report_address      string `json:"report_address,omitempty"`
	Switch_m3u8         bool   `json:"switch_m3u8,omitempty"`
}

type ReceiveBody struct {
	Check_result       string              `json:"check_result,omitempty" reported: computed it and report`
	Check_value        string              `json:"check_value,omitempty" receiverd: last step of dispatch`
	Download_mean_rate string              `json:"download_mean_rate,omitempty"`
	Lvs_address        string              `json:"lvs_address,omitempty"`
	Preload_status     string              `json:"preload_status,omitempty"`
	Sessionid          string              `json:"sessionid,omitempty"`
	Http_status        int                 `json:"http_status,omitempty"`
	Response_time      string              `json:"response_time,omitempty"`
	Refresh_status     string              `json:"refresh_status,omitempty"`
	Check_type         string              `json:"check_type,omitempty" "BASIC" or "MD5"`
	Data               string              `json:"data,omitempty"`
	Last_modified      string              `json:"last_modified,omitempty"`
	Url                string              `json:"url,omitempty"`
	Cache_status       string              `json:"cache_status,omitempty"`
	Url_id             string              `json:"url_id,omitempty"`
	Content_length     int                 `json:"content_length,omitempty"`
	Report_ip          string              `json:"report_ip,omitempty"`
	Report_port        string              `json:"report_port,omitempty"`
	Origin_Remote_IP   string              `json:"origin_remote_ip,omitempty"`
	Status             string              `json:"status,omitempty"`
	Is_compressed      bool                `json:"is_compressed,omitempty"`
	Rate               int                 `json:"rate,omitempty"`
	Conn               int                 `json:"conn,omitempty"`
	Priority           int                 `json:"priority,omitempty"`
	Header_list        []map[string]string `json:"header_list,omitempty"`
	Switch_m3u8        bool                `json:"switch_m3u8,omitempty" whether do little ts in m3u8 file`
}

type ConnInStruct struct {
	Url    string
	Url_id string
	Conn   int
	Status string `UNPROCESSED`
}

type ConnOutStruct struct {
	Url    string
	Url_id string
	Conn   int
	Status string `record the status whether finished.`
}

type ConnStruct struct {
	TasksIn  []ConnInStruct
	InNum    int
	ConnChan chan int
	TasksOut []ConnOutStruct
	OutNum   int
	// Otype    string
}

type ConnTasks struct {
	mu sync.RWMutex
	// mu    sync.Mutex
	Tasks map[string]ConnStruct
}

func (ct *ConnTasks) read() string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return fmt.Sprintf("%+v", ct)
}

func (ct *ConnTasks) write(conn_map map[string]ConnStruct) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	var conn_struct ConnStruct
	if ct.Tasks == nil {
		log.Debugf("ConnTasks write [ct.Tasks == nil] ct: %+v", ct)
		ct.Tasks = make(map[string]ConnStruct)
	}
	for k, v := range conn_map {
		task, is_existed := ct.Tasks[k]
		if !is_existed {
			conn_struct.TasksIn = append(conn_struct.TasksIn, v.TasksIn...)
			conn_struct.InNum += v.InNum
			conn_struct.ConnChan = v.ConnChan
			conn_struct.TasksOut = append(conn_struct.TasksOut, v.TasksOut...)
			conn_struct.OutNum += v.OutNum
			ct.Tasks[k] = conn_struct
			log.Debugf("ConnTasks write [k: %s notExisted.] ct: %+v", k, ct)
		} else {
			task.TasksIn = append(task.TasksIn, v.TasksIn...)
			task.InNum += v.InNum
			task.TasksOut = append(task.TasksOut, v.TasksOut...)
			task.OutNum += v.OutNum
			ct.Tasks[k] = task
		}
	}
}

var ConnTaskStruct ConnTasks

type M3U8_struct struct {
	uid          string
	fetch_num    int
	url_filtered []string
}

type Ts_struct struct {
	Ts_uid      string
	Url         string
	Http_status int
	Body_length int
	Status      string
}

// [ ack format as follow:
// {'sessionid': '089790c08e8911e1910800247e10b29b',
// 'pre_ret_list': [{'id': 'sfsdf', 'code': 200}]}
// ]

func NewReceiveBody(data *DataFromCenter) ([]ReceiveBody, map[string]interface{}) {
	var pre_ret = make(map[string]interface{})
	var pre_ret_list []map[string]interface{}
	var ack = make(map[string]interface{})
	var result []ReceiveBody
	var is_compressed bool
	var body ReceiveBody
	var url_list []Task
	if len(data.Compressed_url_list) > 0 {
		is_compressed = true
		url_list = data.Compressed_url_list
	}
	if len(data.Url_list) > 0 {
		is_compressed = false
		url_list = data.Url_list
	}
	log.Debugf("NewReceiveBody type(url_list): %T|| url_list: %+v", url_list, url_list)
	for i := 0; i < len(url_list); i++ {
		body.Last_modified = "-"
		body.Response_time = "-"
		body.Download_mean_rate = "0"
		body.Http_status = 0
		body.Preload_status = "200"
		body.Refresh_status = "failed"
		body.Data = "-"
		body.Cache_status = "HIT"
		body.Content_length = 0
		body.Is_compressed = is_compressed

		body.Lvs_address = data.Lvs_address
		body.Sessionid = data.Sessionid
		body.Switch_m3u8 = data.Switch_m3u8
		body.Report_ip, body.Report_port = splitReportAddress(data.Report_address)
		body.Url_id = url_list[i].Id
		body.Url = url_list[i].Url
		body.Check_type = url_list[i].Check_type
		// body.Check_result = url_list[i].Md5
		body.Rate = url_list[i].Rate
		body.Conn = url_list[i].Conn
		body.Origin_Remote_IP = url_list[i].Origin_Remote_IP
		body.Priority = url_list[i].Priority
		body.Header_list = url_list[i].Header_list
		result = append(result, body)

		pre_ret["id"] = url_list[i].Id
		pre_ret["code"] = 200
		pre_ret_list = append(pre_ret_list, pre_ret)
	}
	ack["sessionid"] = data.Sessionid
	ack["pre_ret_list"] = pre_ret_list
	return result, ack
}

func splitReportAddress(report_address string) (string, string) {
	report_arr := strings.SplitN(report_address, ":", 2)
	return report_arr[0], report_arr[1]
}

// judge url http or https
func is_http(str string) (is_http_t bool) {
	array_str := strings.SplitN(str, "/", 4)
	is_http_t = true
	if len(array_str) > 0 {
		// log.Debugf("is_http url: %s|| array_str[0]: %s", str, array_str[0])
		if array_str[0] == "https:" {
			is_http_t = false
		}

	}
	return is_http_t
}

func getChannelName(url string) string {
	url_arr := strings.Split(url, "/")
	channelName := url_arr[0] + "//" + url_arr[2]
	return channelName
}

func get_head_length(str string) (string, int, int, int) {
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
	}
	// 当Content-Range存在时，/后面的数字是全部内容的长度，这时的Content-Length也是存在的，但表示的是此次range请求要了多长。
	if has_cr {
		for _, s_line := range every_line_arr {
			if strings.Contains(s_line, "Content-Range:") {
				theContentLength, err = strconv.Atoi(strings.Replace(strings.SplitN(s_line, "/", 2)[1], " ", "", -1))
				if err != nil {
					log.Debugf("get_head_length[theContentRength convert to int err]: %s|| s_line: %s", err, s_line)
				}
			}
		}
	} else if !has_cr && has_cl {
		for _, s_line := range every_line_arr {
			if strings.Contains(s_line, "Content-Length:") {
				theContentLength, err = strconv.Atoi(strings.Replace(strings.SplitN(s_line, ":", 2)[1], " ", "", -1))
				if err != nil {
					log.Debugf("get_head_length theContentLength convert to int err: %s|| s_line: %s", err, s_line)
				}
			}
		}

	}
	http_status, err := strconv.Atoi(strings.SplitN(first_line, " ", 3)[1])
	if err != nil {
		log.Debugf("get_head_length http_status convert to int error.http_status: %s", err)
	}
	head_length := len(thehead)
	return thehead, head_length, http_status, theContentLength
}

func check_conn_out(body ReceiveBody) {
	connKey := getChannelName(body.Url) + "__" + fmt.Sprintf("%d", body.Conn)

	var conn_out_struct ConnOutStruct
	conn_out_struct.Url_id = body.Url_id
	conn_out_struct.Url = body.Url
	conn_out_struct.Conn = body.Conn
	conn_out_struct.Status = body.Status

	var conn_struct ConnStruct
	conn_struct.OutNum += 1
	conn_struct.TasksOut = append(conn_struct.TasksOut, conn_out_struct)

	var conn_map = make(map[string]ConnStruct)
	conn_map[connKey] = conn_struct

	ConnTaskStruct.write(conn_map)
	log.Debugf("check_conn_out ConnTaskStruct.read(): %+v|| Url_id: %s|| Url: %s", ConnTaskStruct.read(), body.Url_id, body.Url)

	connOut := <-ConnConcurrent_ch
	log.Debugf("check_conn_out [connOut: %+v out!] Url_id: %s|| Url: %s", connOut, body.Url_id, body.Url)
	connTaskOut := <-ConnTaskStruct.Tasks[connKey].ConnChan
	log.Debugf("check_conn_out [connTaskOut: %+v out!!] Url_id: %s|| Url: %s", connTaskOut, body.Url_id, body.Url)

	if ConnTaskStruct.Tasks[connKey].InNum == ConnTaskStruct.Tasks[connKey].OutNum {
		delete(ConnTaskStruct.Tasks, connKey)
		log.Debugf("check_conn_out [NumIn==NumOut,delete the key successfully.]|| connKey: %s|| ConnTaskStruct.read(): %+v|| Url_id: %s|| Url: %s", connKey, ConnTaskStruct.read(), body.Url_id, body.Url)
	}
}

func ProxyRangeRequest(ip_to string, head string, url_id string, url string, retry_num int, read_start int, read_end int, file_total_length int) (int, int, int) {
	RangeConcurrent_ch <- 1
	channel_name := getChannelName(url)
	var partial_num int

	for r_num := 0; r_num < retry_num; r_num++ {
		var partial_size int = PartialSize
		var last_length int = (read_end + 1) % PartialSize
		partial_num = (read_end + 1) / PartialSize

		// 最后一个分片,使用read_end == 文件总长度来标识
		if read_end+1 == file_total_length {
			partial_num = -1
			if last_length != 0 {
				partial_size = last_length
			}
		}

		// 文件大小小于分片大小的情况
		if read_start == 0 && file_total_length < PartialSize {
			partial_size = file_total_length
		}

		tcpAddr, errResolveTCPAddr := net.ResolveTCPAddr("tcp", ip_to)
		if errResolveTCPAddr != nil {
			log.Errorf("ProxyRangeRequest [errResolveTCPAddr] inRangeIndex(%d|| [%d, %d]) [retried No.%d times] errResolveTCPAddr: %s", partial_num, read_start, read_end, r_num, errResolveTCPAddr)
		}

		tcpconn, errDialTCP := net.DialTCP("tcp", nil, tcpAddr)
		if errDialTCP != nil {
			log.Errorf("ProxyRangeRequest [errDialTCP] inRangeIndex(%d|| [%d, %d]) [retried No.%d times] errDialTCP: %s", partial_num, read_start, read_end, r_num, errDialTCP)
		}

		str_t := fmt.Sprintf("GET %s HTTP/1.1\r\nRange: bytes=%d-%d\r\nHost: %s\r\n%s\r\nAccept: */*\r\nProxy-Connection: Keep-Alive\r\n\r\n", url, read_start, read_end, channel_name[7:], head)
		log.Debugf("ProxyRangeRequest inRangeIndex(%d|| [%d, %d]) [retried No.%d times] str_t: %s|| partial_size: %d", partial_num, read_start, read_end, r_num, str_t, partial_size)
		//向tcpconn中写入数据
		_, errWrite := tcpconn.Write([]byte(str_t))
		if errWrite != nil {
			log.Errorf("ProxyRangeRequest [errWrite] inRangeIndex(%d|| [%d, %d]) [retried No.%d times] errWrite: %s", partial_num, read_start, read_end, r_num, errWrite)
		}
		buf := make([]byte, 8192)
		tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))

		head_end := false
		var (
			thehead                 string
			h_length                int
			http_status             int
			original_content_length int
			o_content_length        int
			total_length            int
			head_length             int
		)

		for {
			length, errRead := tcpconn.Read(buf)
			if errRead != nil {
				tcpconn.Close()
				log.Debugf("ProxyRangeRequest inRangeIndex(%d|| [%d, %d]) [retried No.%d times] errRead: %s", partial_num, read_start, read_end, r_num, errRead)
				real_body_length := total_length - head_length - 4
				<-RangeConcurrent_ch
				if real_body_length < 0 {
					log.Debugf("ProxyRangeRequest inRangeIndex(%d|| [%d, %d]) [retried No.%d times] [real_body_length < 0 return 0 on purpose.] real_body_length: %d", partial_num, read_start, read_end, r_num, real_body_length)
					return 0, original_content_length, http_status
				}
				return real_body_length, original_content_length, http_status
			}

			if !head_end {
				recvStr := string(buf[:length])
				is_head_end := strings.Contains(recvStr, "\r\n\r\n")
				if !is_head_end {
					head_length += length
				} else {
					thehead, h_length, http_status, original_content_length = get_head_length(recvStr)
					o_content_length = original_content_length
					head_length += h_length
					head_end = true
					log.Debugf("ProxyRangeRequest inRangeIndex(%d|| [%d, %d]) [retried No.%d times] o_content_length: %d|| head_length: %d|| http_status: %d|| thehead: %s", partial_num, read_start, read_end, r_num, o_content_length, head_length, http_status, thehead)
				}
			}

			if length > 0 {
				total_length += length
				tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))
			}

			// the length received satisfied with Content_Length, close connection initiatively.
			if (o_content_length > 0) && ((total_length - head_length - 4) >= partial_size) {
				tcpconn.Close()
				log.Debugf("ProxyRangeRequest inRangeIndex(%d|| [%d, %d]) [retried No.%d times] [the length of receiving satisfied with , Content_Length close connection initiatively.] total_length: %d|| head_length: %d|| o_content_length: %d", partial_num, read_start, read_end, r_num, total_length, head_length, o_content_length)
				<-RangeConcurrent_ch
				return (total_length - head_length - 4), o_content_length, http_status
			}
		}
	}
	return 0, -1, 0
}

func ProxyRequest(ip_to string, head string, url_id string, url string, read_flag bool) string {
	channel_name := getChannelName(url)
	tcpAddr, errResolveTCPAddr := net.ResolveTCPAddr("tcp", ip_to)
	if errResolveTCPAddr != nil {
		log.Debugf("ProxyRequest errResolveTCPAddr: %s", errResolveTCPAddr)
	}

	tcpconn, errDialTCP := net.DialTCP("tcp", nil, tcpAddr)
	if errDialTCP != nil {
		log.Debugf("ProxyRequest errDialTCP: %s", errDialTCP)
	}

	// str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + head + conn_header + "\r\n\r\n"
	str_t := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\n%s\r\nAccept: */*\r\nProxy-Connection: Keep-Alive\r\n\r\n", url, channel_name[7:], head)
	log.Debugf("ProxyRequest str_t: %s", str_t)
	//向tcpconn中写入数据
	_, errWrite := tcpconn.Write([]byte(str_t))
	if errWrite != nil {
		log.Debugf("ProxyRequest errWrite: %s", errWrite)
	}
	buf := make([]byte, 8192)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))

	head_end := false
	var (
		content                 string
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		total_length            int
		head_length             int
	)

	for {
		length, errRead := tcpconn.Read(buf)
		if errRead != nil {
			tcpconn.Close()
			log.Debugf("ProxyRequest errRead: %s", errRead)
			if read_flag {
				return content
			}
			break
		}

		if !head_end {
			recvStr := string(buf[:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = get_head_length(recvStr)
				o_content_length = original_content_length
				head_length += h_length
				head_end = true
				log.Debugf("ProxyRequest o_content_length: %d|| head_length: %d|| http_status: %d|| thehead: %s", o_content_length, head_length, http_status, thehead)
			}
		}

		if length > 0 {
			total_length += length
			if read_flag {
				content += string(buf[:length])
			}
			tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))
		}

		// the length received satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((total_length - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			log.Debugf("ProxyRequest [the length of receiving satisfied with Content_Length, close connection initiatively.] total_length: %d|| head_length: %d|| o_content_length: %d", total_length, head_length, o_content_length)
			if read_flag {
				return content
			}
			break
		}
	}
	return "ProxyRequest FINISHED."
}

func ltsProxyRequest(ip_to string, head string, url_id string, url string, is_little_ts bool, countRainbowChan chan int, TsOutputChan chan Ts_struct) {
	channel_name := getChannelName(url)
	tcpAddr, errResolveTCPAddr := net.ResolveTCPAddr("tcp", ip_to)
	if errResolveTCPAddr != nil {
		log.Debugf("ltsProxyRequest errResolveTCPAddr: %s", errResolveTCPAddr)
	}

	tcpconn, errDialTCP := net.DialTCP("tcp", nil, tcpAddr)
	if errDialTCP != nil {
		log.Debugf("ltsProxyRequest errDialTCP: %s", errDialTCP)
	}

	// str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + head + conn_header + "\r\n\r\n"
	str_t := fmt.Sprintf("GET %s&vkey=3039134e23ab17addeff8979e8bc5e43 HTTP/1.1\r\nHost: %s\r\n%s\r\nAccept: */*\r\nProxy-Connection: Keep-Alive\r\n\r\n", url, channel_name[7:], head)
	log.Debugf("ltsProxyRequest str_t: %s", str_t)
	//向tcpconn中写入数据
	_, errWrite := tcpconn.Write([]byte(str_t))
	if errWrite != nil {
		log.Debugf("ltsProxyRequest errWrite: %s", errWrite)
	}
	buf := make([]byte, 8192)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))

	body_length := 0
	head_end := false
	var (
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		total_length            int
		head_length             int
	)

	for {
		length, errRead := tcpconn.Read(buf)
		if errRead != nil {
			tcpconn.Close()
			log.Debugf("ltsProxyRequest errRead: %s|| url: %s", errRead, url)
			if errRead == io.EOF {
				if is_little_ts {
					var ts_st Ts_struct
					ts_st.Ts_uid = url_id
					ts_st.Url = url
					ts_st.Http_status = http_status // HTTP status code from header
					ts_st.Body_length = body_length // Content-Length from header
					ts_st.Status = "Done..."
					TsOutputChan <- ts_st
					countRainbowChan <- 1
					<-TsConcurrent_ch
				}
			}
			break
		}

		if !head_end {
			recvStr := string(buf[:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = get_head_length(recvStr)
				o_content_length = original_content_length
				head_length += h_length
				head_end = true
				log.Debugf("ltsProxyRequest url: %s o_content_length: %d|| head_length: %d|| http_status: %d|| thehead: %s", url, o_content_length, head_length, http_status, thehead)
			}
		}

		if length > 0 {
			total_length += length
			tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))
		}

		// the length received satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((total_length - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			log.Debugf("ltsProxyRequest url: %s|| [the length of receiving satisfied with Content_Length, close connection initiatively.] total_length: %d|| head_length: %d|| o_content_length: %d", url, total_length, head_length, o_content_length)
			body_length = total_length - head_length - 4
			log.Debugf("ltsProxyRequest body_length: %d", body_length)
			if is_little_ts {
				var ts_st Ts_struct
				ts_st.Ts_uid = url_id
				ts_st.Url = url
				ts_st.Http_status = http_status // HTTP status code from header
				ts_st.Body_length = body_length // Content-Length from header
				ts_st.Status = "Done.."
				countRainbowChan <- 1
				TsOutputChan <- ts_st
				<-TsConcurrent_ch
				log.Debugf("ltsProxyRequest url: %s|| ts_st: %+v", url, ts_st)
			}
			break
		}
	}
}

func GetProxy_http(ip_to string, head string, receiveBody_t ReceiveBody) {
	// channel_name := getChannelName(receiveBody_t.Url)
	tcpAddr, errResolveTCPAddr := net.ResolveTCPAddr("tcp", ip_to)
	if errResolveTCPAddr != nil {
		log.Debugf("GetProxy_http errResolveTCPAddr: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", errResolveTCPAddr, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
	}

	tcpconn, errDialTCP := net.DialTCP("tcp", nil, tcpAddr)
	if errDialTCP != nil {
		log.Debugf("GetProxy_http errDialTCP: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", errDialTCP, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
		receiveBody_t.Status = fmt.Sprintf("%s", errDialTCP)
		PostRealBody(receiveBody_t)
		if receiveBody_t.Conn != 0 {
			check_conn_out(receiveBody_t)
		}
		<-MaxConcurrent_ch
		return
	}
	host := GetHost(receiveBody_t.Url)
	var headers_str string
	for _, d := range receiveBody_t.Header_list {
		for k, v := range d {
			headers_str += k + ":" + v + "\r\n"
		}
	}
	log.Debugf("GetProxy_http headers_str: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", headers_str, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)

	var str_t string
	var conn_header string
	if receiveBody_t.Conn != 0 {
		conn_header = fmt.Sprintf("\r\nCC_PRELOAD_SPEED:%dKB", receiveBody_t.Rate)
	}
	if receiveBody_t.Is_compressed {
		str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + "Accept-Encoding:gzip\r\n" + head + conn_header + "\r\n\r\n"
	} else {
		str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + head + conn_header + "\r\n\r\n"
	}

	log.Debugf("GetProxy_http str_t: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", str_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
	//向tcpconn中写入数据
	_, err3 := tcpconn.Write([]byte(str_t))
	if err3 != nil {
		log.Debugf("GetProxy_http err3: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", err3, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
		receiveBody_t.Status = fmt.Sprintf("%s", err3)
		PostRealBody(receiveBody_t)
		if receiveBody_t.Conn != 0 {
			check_conn_out(receiveBody_t)
		}
		<-MaxConcurrent_ch
		return

	}
	buf := make([]byte, 8192)
	log.Debugf("GetProxy_http readTimeout: %d|| url_id: %s|| origin_remote_ip: %s|| url: %s", readTimeout, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))

	head_end := false
	var (
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		total_length            int
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
			log.Debugf("GetProxy_http errRead: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", errRead, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			// get Content_length and Download_mean_rate
			receiveBody_t.Content_length = total_length - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if receiveBody_t.Content_length < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(receiveBody_t.Content_length) / 1024
				} else {
					d_rate = float64(receiveBody_t.Content_length) / 1024 / float64(delta_t)
				}
			}
			receiveBody_t.Download_mean_rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			log.Debugf("GetProxy_http total_length: %d|| delta_t: %f|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, delta_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			if errRead == io.EOF {
				receiveBody_t.Status = "Done"
			} else if strings.Contains(fmt.Sprintf("%s", errRead), "timeout") {
				receiveBody_t.Status = fmt.Sprintf("Timeout(%ds)", readTimeout)
				if receiveBody_t.Content_length <= 0 {
					receiveBody_t.Content_length = 0
				}
			} else {
				receiveBody_t.Status = fmt.Sprintf("%s", errRead)
			}
			PostRealBody(receiveBody_t)
			if receiveBody_t.Conn != 0 {
				check_conn_out(receiveBody_t)
			}
			<-MaxConcurrent_ch
			break
		}

		if length > 0 {
			total_length += length
			tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))
		}

		if !head_end {
			recvStr := string(buf[:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = get_head_length(recvStr)
				o_content_length = original_content_length
				head_length += h_length
				head_end = true
				receiveBody_t.Http_status = http_status
				log.Debugf("GetProxy_http o_content_length: %d|| head_length: %d|| http_status: %d|| thehead: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", o_content_length, head_length, http_status, thehead, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			}
		}

		// body begin.make the md5 checksum.
		if receiveBody_t.Check_type == "MD5" && total_length >= head_length+4 {
			if !body_first {
				body_first = true
				body_first_start = head_length + 4
				log.Debugf("GetProxy_http body_first_start: %d|| url_id: %s|| origin_remote_ip: %s|| url: %s", body_first_start, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
				if body_first_start != 0 {
					md5Ctx.Write(buf[body_first_start:length])
				}
			} else {
				md5Ctx.Write(buf[:length])
			}
		}

		// the length of receiving satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((total_length - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			log.Debugf("GetProxy_http [the length of receiving satisfied with Content_Length, close connection initiatively.] total_length: %d|| head_length: %d|| o_content_length: %d|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, head_length, o_content_length, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			// get Content_length and Download_mean_rate
			receiveBody_t.Content_length = total_length - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if receiveBody_t.Content_length < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(receiveBody_t.Content_length) / 1024
				} else {
					d_rate = float64(receiveBody_t.Content_length) / 1024 / float64(delta_t)
				}
			}
			receiveBody_t.Download_mean_rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			log.Debugf("GetProxy_http total_length: %d|| delta_t: %f|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, delta_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			receiveBody_t.Status = "Done."
			if receiveBody_t.Check_type == "MD5" {
				// output md5 checksum
				cipherStr := md5Ctx.Sum(nil)
				receiveBody_t.Check_result = hex.EncodeToString(cipherStr)
			}
			if receiveBody_t.Conn != 0 {
				check_conn_out(receiveBody_t)
			}
			PostRealBody(receiveBody_t)
			<-MaxConcurrent_ch
			break
		}
	}
}

// https
func GetProxy_https(ip_to string, head string, receiveBody_t ReceiveBody) {
	// channel_name := getChannelName(receiveBody_t.Url)
	host := GetHost(receiveBody_t.Url)
	var headers_str string
	for _, d := range receiveBody_t.Header_list {
		for k, v := range d {
			headers_str += k + ":" + v + "\r\n"
		}
	}
	log.Debugf("GetProxy_https headers_str: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", headers_str, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)

	var str_t string
	var conn_header string
	if receiveBody_t.Conn != 0 {
		conn_header = fmt.Sprintf("\r\nCC_PRELOAD_SPEED:%dKB", receiveBody_t.Rate)
	}
	if receiveBody_t.Is_compressed {
		str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + "Accept-Encoding:gzip\r\n" + head + conn_header + "\r\n\r\n"
	} else {
		str_t = "GET " + receiveBody_t.Url + " HTTP/1.1\r\n" + "Host:" + host + "\r\n" + headers_str + head + conn_header + "\r\n\r\n"
	}
	log.Debugf("GetProxy_https str_t: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", str_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	tcpconn, err2 := tls.Dial("tcp", sslIpTo, conf)
	if err2 != nil {
		log.Debugf("GetProxy_https err2: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", err2, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
		receiveBody_t.Status = fmt.Sprintf("%s", err2)
		PostRealBody(receiveBody_t)
		if receiveBody_t.Conn != 0 {
			check_conn_out(receiveBody_t)
		}
		<-MaxConcurrent_ch
		return
	}

	//向tcpconn中写入数据
	_, err3 := tcpconn.Write([]byte(str_t))
	if err3 != nil {
		log.Debugf("GetProxy_https err3: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", err3, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
		receiveBody_t.Status = fmt.Sprintf("%s", err3)
		PostRealBody(receiveBody_t)
		if receiveBody_t.Conn != 0 {
			check_conn_out(receiveBody_t)
		}
		<-MaxConcurrent_ch
		return
	}
	buf := make([]byte, 8192)
	log.Debugf("GetProxy_https readTimeout: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", readTimeout, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
	tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))

	head_end := false
	var (
		thehead                 string
		h_length                int
		http_status             int
		original_content_length int
		o_content_length        int
		total_length            int
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
			log.Debugf("GetProxy_https err: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", err, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			receiveBody_t.Content_length = total_length - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if receiveBody_t.Content_length < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(receiveBody_t.Content_length) / 1024
				} else {
					d_rate = float64(receiveBody_t.Content_length) / 1024 / float64(delta_t)
				}
			}
			receiveBody_t.Download_mean_rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			log.Debugf("GetProxy_https total_length: %s|| delta_t: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, delta_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			if err == io.EOF {
				receiveBody_t.Status = "Done"
			} else if strings.Contains(fmt.Sprintf("%s", err), "timeout") {
				receiveBody_t.Status = fmt.Sprintf("Timeout(%ds)", readTimeout)
				if receiveBody_t.Content_length <= 0 {
					receiveBody_t.Content_length = 0
				}
			} else {
				receiveBody_t.Status = fmt.Sprintf("%s", err)
			}
			PostRealBody(receiveBody_t)
			if receiveBody_t.Conn != 0 {
				check_conn_out(receiveBody_t)
			}
			<-MaxConcurrent_ch
			break
		}
		if length > 0 {
			total_length += length
			tcpconn.SetDeadline(time.Now().Add(time.Duration(readTimeout) * time.Second))
		}
		if !head_end {
			recvStr := string(buf[0:length])
			is_head_end := strings.Contains(recvStr, "\r\n\r\n")
			if !is_head_end {
				head_length += length
			} else {
				thehead, h_length, http_status, original_content_length = get_head_length(recvStr)
				o_content_length = original_content_length
				receiveBody_t.Http_status = http_status
				head_length += h_length
				head_end = true
				log.Debugf("GetProxy_https o_content_length: %d|| head_length: %s|| http_status: %s|| thehead: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", o_content_length, head_length, http_status, thehead, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			}
		}

		// body begin.make the md5 checksum.
		if receiveBody_t.Check_type == "MD5" && total_length >= head_length+4 {
			if !body_first {
				body_first = true
				body_first_start = head_length + 4
				log.Debugf("GetProxy_https body_first_start: %d|| url_id: %s|| origin_remote_ip: %s|| url: %s", body_first_start, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
				if body_first_start != 0 {
					md5Ctx.Write(buf[body_first_start:length])
				}
			} else {
				md5Ctx.Write(buf[:length])
			}
		}

		// the length of receiving satisfied with Content_Length, close connection initiatively.
		if (o_content_length > 0) && ((total_length - head_length - 4) >= o_content_length) {
			tcpconn.Close()
			log.Debugf("GetProxy_https the length of receiving satisfied with Content_Length, close connection initiatively. total_length: %d|| head_length: %d|| o_content_length: %d|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, head_length, o_content_length, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			// get Content_length and Download_mean_rate
			receiveBody_t.Content_length = total_length - head_length - 4
			delta_t := (float64(time.Now().UnixNano()/1e6) - st) / 1000
			var d_rate float64
			if receiveBody_t.Content_length < 0 {
				d_rate = 0
			} else {
				if delta_t <= 0 {
					d_rate = float64(receiveBody_t.Content_length) / 1024
				} else {
					d_rate = float64(receiveBody_t.Content_length) / 1024 / float64(delta_t)
				}
			}
			receiveBody_t.Download_mean_rate = strconv.FormatFloat(d_rate, 'f', 6, 64)
			log.Debugf("GetProxy_https total_length: %d|| delta_t: %f|| url_id: %s|| origin_remote_ip: %s|| url: %s", total_length, delta_t, receiveBody_t.Url_id, receiveBody_t.Origin_Remote_IP, receiveBody_t.Url)
			receiveBody_t.Status = "Done."
			PostRealBody(receiveBody_t)
			if receiveBody_t.Conn != 0 {
				check_conn_out(receiveBody_t)
			}
			<-MaxConcurrent_ch
			break
		}
	}
}

func Process_go(body ReceiveBody) {
	if body.Conn == 0 {
		log.Infof("Process_go cap(MaxConcurrent_ch): %v|| len(MaxConcurrent_ch): %v", cap(MaxConcurrent_ch), len(MaxConcurrent_ch))
		MaxConcurrent_ch <- 1
		log.Infof("Process_go cap(MaxConcurrent_ch): %v|| len(MaxConcurrent_ch): %v", cap(MaxConcurrent_ch), len(MaxConcurrent_ch))
	}
	is_http_t := is_http(body.Url)
	log.Debugf("Process_go Url_id: %s|| origin_remote_ip: %s|| url: %s|| ipTo: %s|| HeadSend: %s|| is_http_t: %t", body.Url_id, body.Origin_Remote_IP, body.Url, ipTo, HeadSend, is_http_t)
	if is_http_t == true {
		GetProxy_http(ipTo, HeadSend, body)
	} else {
		GetProxy_https(ipTo, HeadSend, body)
	}
	log.Debugf("Process_go [finished] Url_id: %s|| origin_remote_ip: %s|| status: %s|| Url: %s", body.Url_id, body.Origin_Remote_IP, body.Status, body.Url)
}

func ProcessConnTask(body ReceiveBody) {
	ConnConcurrent_ch <- 1
	// init the conn struct
	conn := body.Conn
	channel_name := getChannelName(body.Url)
	connKey := channel_name + "__" + fmt.Sprintf("%d", conn)

	var conn_in_struct ConnInStruct
	conn_in_struct.Url_id = body.Url_id
	conn_in_struct.Url = body.Url
	conn_in_struct.Conn = body.Conn
	conn_in_struct.Status = "UNPROCESSED"

	var conn_struct ConnStruct
	conn_struct.TasksIn = append(conn_struct.TasksIn, conn_in_struct)
	conn_struct.InNum += 1

	_, is_existed := ConnTaskStruct.Tasks[connKey]
	if !is_existed {
		log.Debugf("ProcessConnTask [key: %s notExisted.] ConnTaskStruct.read(): %+v|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", connKey, ConnTaskStruct.read(), body.Url_id, body.Origin_Remote_IP, body.Url)
		conn_struct.ConnChan = make(chan int, conn)
	}

	var conn_map = make(map[string]ConnStruct)
	conn_map[connKey] = conn_struct

	ConnTaskStruct.write(conn_map)

	ConnTaskStruct.Tasks[connKey].ConnChan <- 1

	log.Debugf("ProcessConnTask [Initialization done.] ConnTaskStruct.read(): %+v|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", ConnTaskStruct.read(), body.Url_id, body.Origin_Remote_IP, body.Url)

	Process_go(body)
}

func RangeTask(body ReceiveBody) {
	var (
		http_status         int
		read_length         int
		target_total_length int
		total_read_length   int
	)
	// 请求0-1分片确定文件总大小,不可省,否则无法确定给定文件大小与分片的大小关系.
	_, target_total_length, http_status = ProxyRangeRequest(ipTo, HeadSend, body.Url_id, body.Url, 10, 0, 1, target_total_length)
	log.Debugf("RangeTask [from ProxyRangeRequest] PartialSize: %d|| target_total_length: %d|| http_status: %d|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", PartialSize, target_total_length, http_status, body.Url_id, body.Origin_Remote_IP, body.Url)

	if http_status != 206 && http_status != 200 {
		log.Errorf("RangeTask [Resources Code Error.] http_status: %d|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", http_status, body.Url_id, body.Origin_Remote_IP, body.Url)
		return
	}

	var loop_num int = target_total_length / PartialSize
	var last_length int = target_total_length % PartialSize
	var range_ch = make(chan int, loop_num)
	if last_length != 0 {
		loop_num++ // 考虑 100 / 10 和 101 / 10的情况
	}

	for i := 1; i <= loop_num; i++ {
		if i == 1 && target_total_length < PartialSize { // 文件大小 < 分片大小
			go func(n int) {
				read_length, _, _ = ProxyRangeRequest(ipTo, HeadSend, body.Url_id, body.Url, 10, (n-1)*PartialSize, target_total_length-1, target_total_length)
				range_ch <- read_length
				log.Debugf("RangeTask [target_total_length lt PartialSize(%d)] RangeIndex(%d)[%d, %d] Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", PartialSize, n, (n-1)*PartialSize, target_total_length, body.Url_id, body.Origin_Remote_IP, body.Url)
			}(i)
		} else if i == loop_num && last_length != 0 && target_total_length >= PartialSize { // 文件大小 > 分片大小,并且最后一个分片
			go func(n int) {
				read_length, _, _ = ProxyRangeRequest(ipTo, HeadSend, body.Url_id, body.Url, 10, (n-1)*PartialSize, (n-1)*PartialSize+last_length-1, target_total_length)
				range_ch <- read_length
				log.Debugf("RangeTask [LastPartial.] RangeIndex(%d)[%d, %d] Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", n, body.Origin_Remote_IP, (n-1)*PartialSize, (n-1)*PartialSize+last_length, body.Url_id, body.Url)
			}(i)
		} else {
			go func(n int) {
				read_length, _, _ = ProxyRangeRequest(ipTo, HeadSend, body.Url_id, body.Url, 10, (n-1)*PartialSize, n*PartialSize-1, target_total_length)
				range_ch <- read_length
				log.Debugf("RangeTask RangeIndex(%d)[%d, %d] Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", n, body.Origin_Remote_IP, (n-1)*PartialSize, n*PartialSize-1, body.Url_id, body.Url)
			}(i)
		}
	}

	for j := 1; j <= loop_num; j++ {
		total_read_length += <-range_ch
		log.Debugf("RangeTask [current/loop_num: (%d/%d)] total_read_length: %d|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", j, loop_num, total_read_length, body.Url_id, body.Origin_Remote_IP, body.Url)
	}

	close(range_ch)
	log.Debugf("RangeTask [range_ch has been closed!] total_read_length: %d|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", total_read_length, body.Url_id, body.Origin_Remote_IP, body.Url)
	if total_read_length >= target_total_length {
		log.Debugf("RangeTask [Congratulations----The big file's preloading is completed!] total_read_length: %d|| Url_id: %+v|| Origin_Remote_IP: %+v|| Url: %+v", total_read_length, body.Url_id, body.Origin_Remote_IP, body.Url)
	}
}

// this function is suitable for qq's m3u8 tasks
func get_m3u8_qq(url_id string, url string) M3U8_struct {
	var theM3VKEY = ".m3u8?vkey=3039134e23ab17addeff8979e8bc5e43"
	var channel_name = getChannelName(url)
	var fetch_num int
	var url_filtered []string
	// var rainbow_map = make(map[string]interface{})
	var m3u8_struct M3U8_struct
	var file_name string
	// log.Debugf("channel_name: %s|| url_id: %s", channel_name, url_id)
	url = strings.TrimSpace(url)
	url = strings.SplitN(url, "?", 2)[0]
	// ex1: http://ltslx.qq.com/w0025mpfimu.321004.1.ts
	// ex2: http://ltslx.qq.com/u0025s9uq4c.320001.ts
	if strings.HasSuffix(url, ".ts") {
		path_part := strings.SplitN(url, "/", 4)[3]
		dotArray := strings.Split(path_part, ".")
		if len(dotArray) == 4 {
			file_name = fmt.Sprintf("%s.%s.%s", dotArray[0], dotArray[1], dotArray[3])
		} else if len(dotArray) == 3 {
			file_name = fmt.Sprintf("%s.%s.%s", dotArray[0], dotArray[1], dotArray[2])
		}
		url_m3u8 := fmt.Sprintf("%s/%s%s", channel_name, file_name, theM3VKEY)
		log.Debugf("get_m3u8_qq url_id: %s|| url: %s|| path_part: %s|| file_name: %s|| url_m3u8: %s", url_id, url, path_part, file_name, url_m3u8)
		file_m3u8 := ProxyRequest(ipTo, HeadSend, url_id, url_m3u8, true)
		log.Debugf("get_m3u8_qq url_id: %s|| url: %s|| len(file_m3u8): %d", url_id, url, len(file_m3u8))
		for _, url_front_all := range strings.Split(file_m3u8, "\n") {
			if strings.Contains(url_front_all, path_part) {
				url_ts := url_front_all
				fetch_num += 1
				url_filtered = append(url_filtered, fmt.Sprintf("%s/%s", channel_name, url_ts))
			}
		}
		m3u8_struct.uid = url_id
		m3u8_struct.fetch_num = fetch_num
		m3u8_struct.url_filtered = url_filtered
	}
	return m3u8_struct
}

/*TaskRequestPost 处理实时接收汇报的请求，并异步判断请求 */
func TaskRequestPost(c *gin.Context) {
	var data DataFromCenter
	err := c.ShouldBindJSON(&data)
	if err != nil {
		log.Error("TaskRequestPost err: ", err)
		return
	}

	body, ack := NewReceiveBody(&data) // originally from lua
	log.Debugf("TaskRequestPost RemoteAddr: %s|| data: %+v|| body: %+v|| ack: %+v", c.ClientIP(), data, body, ack)

	// ack reply center
	defer c.JSON(200, ack)

	// process all kinds of tasks.
	for i := 0; i < len(body); i++ {
		if !body[i].Switch_m3u8 {
			if body[i].Priority > 0 {
				go RangeTask(body[i])
			} else {
				if body[i].Conn != 0 {
					go ProcessConnTask(body[i])
				} else {
					go Process_go(body[i])
				}
			}
		} else {
			body[i].Http_status = -1
			go PostRealBody(body[i])
		}
	}
}

func PostRealBody(data ReceiveBody) {
	var (
		request       *http.Request
		errNewRequest error
		response      *http.Response
		errDo         error
		r_status      int
	)

	for i := 0; i < 4; i++ {

		b, err := json.Marshal(data)
		if err != nil {
			log.Debugf("PostRealBody json err:%s", err)
		}
		body := bytes.NewBuffer([]byte(b))
		log.Debugf("PostRealBody body: %+v", body)
		Report_address := "http://" + data.Report_ip + ":" + data.Report_port + "/content/preload_report?id=" + data.Url_id

		//可以通过client中transport的Dial函数,在自定义Dial函数里面设置建立连接超时时长和发送接受数据超时
		client := &http.Client{
			Transport: &http.Transport{
				Dial: func(netw, addr string) (net.Conn, error) {
					conn, err := net.DialTimeout(netw, addr, time.Second*3) //设置建立连接超时
					if err != nil {
						return nil, err
					}
					conn.SetDeadline(time.Now().Add(time.Second * 5)) //设置发送接收数据超时
					return conn, nil
				},
				ResponseHeaderTimeout: time.Second * 2,
			},
		}
		if i > 0 {
			log.Debugf("PostRealBody body: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", body, data.Url_id, data.Origin_Remote_IP, data.Url)
		}
		request, errNewRequest = http.NewRequest("POST", Report_address, body) //提交请求;用指定的方法，网址，可选的主体返回一个新的*Request
		if errNewRequest != nil {
			log.Debugf("PostRealBody errNewRequest: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", errNewRequest, data.Url_id, data.Origin_Remote_IP, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Host", "www.report.com")
		request.Header.Set("Accept", "*/*")
		request.Header.Set("User-Agent", "ChinaCache")
		request.Header.Set("X-CC-Preload-Report", "ChinaCache")
		request.Header.Set("Content-Length", fmt.Sprintf("%d", len(b)))
		request.Header.Set("Connection", "close")

		response, errDo = client.Do(request) //前面预处理一些参数，状态，Do执行发送；处理返回结果;Do:发送请求,
		if errDo != nil {
			log.Debugf("PostRealBody retried No.%d times, errDo: %s|| request: %+v, Report_address: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", i, errDo, request, Report_address, data.Url_id, data.Origin_Remote_IP, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		r_status = response.StatusCode //获取返回状态码，正常是200
		if r_status != 200 {
			log.Debugf("PostRealBody retried No.%d times, r_status: %d, url_id: %s|| origin_remote_ip: %s|| url: %s", i, r_status, data.Url_id, data.Origin_Remote_IP, data.Url)
			time.Sleep(time.Second * time.Duration(RandInt(7)))
			continue
		}
		defer response.Body.Close()
		r_body, errReadAll := ioutil.ReadAll(response.Body)
		if errReadAll != nil {
			log.Debugf("PostRealBody retried No.%d times, errReadAll: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", i, errReadAll, data.Url_id, data.Origin_Remote_IP, data.Url)
			return
		}
		log.Debugf("PostRealBody retried No.%d times, r_status: %d, r_body: %s|| url_id: %s|| origin_remote_ip: %s|| url: %s", i, r_status, r_body, data.Url_id, data.Origin_Remote_IP, data.Url)
		break
	}

	// Parse m3u8 and preload the little ts tasks.
	if data.Switch_m3u8 {
		var countRainbowChan = make(chan int, TsConcurrent)
		var TsOutputChan = make(chan Ts_struct, TsConcurrent)
		m3u8_struct := get_m3u8_qq(data.Url_id, data.Url)

		log.Debugf("[lts tasks processing.] m3u8_struct: %+v|| origin_remote_ip: %s|| url_id: %+v", m3u8_struct, data.Origin_Remote_IP, data.Url_id)

		for _, url_ts := range m3u8_struct.url_filtered {
			TsConcurrent_ch <- 1
			go ltsProxyRequest(ipTo, HeadSend, data.Url_id, url_ts, true, countRainbowChan, TsOutputChan)
		}
		log.Debugf("[lts tasks processing.] go ltsProxyRequest(ipTo, HeadSend, data.Url_id, url_ts, true, countRainbowChan, TsOutputChan)")

		donelts := 0
		for numlts := range countRainbowChan {
			func(num int) {
				donelts += num
			}(numlts)
			if donelts == m3u8_struct.fetch_num {
				close(countRainbowChan)
			}
		}

		log.Debugf("[lts tasks processing.] donelts: %d", donelts)

		var lts_info []Ts_struct
		for m3_st := range TsOutputChan {
			func(m3_st Ts_struct) {
				lts_info = append(lts_info, m3_st)
			}(m3_st)
			if len(lts_info) == donelts {
				close(TsOutputChan)
			}
		}

		log.Debugf("[lts tasks processing.] url_id: %s|| progress_bar[%d/%d]|| lts_info: %+v", data.Url_id, donelts, m3u8_struct.fetch_num, lts_info)
	}

}
