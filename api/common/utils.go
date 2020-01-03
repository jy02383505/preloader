package common

import (
	"github.com/valyala/fasthttp"
	"net/url"
	"strings"
)

// post 请求
func HttpPost(url string, body string) (int, []byte, error) {

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源

	// 默认是application/x-www-form-urlencoded
	req.Header.SetContentType("application/json")
	req.Header.SetMethod("POST")

	req.SetRequestURI(url)

	requestBody := []byte(body)
	req.SetBody(requestBody)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源

	if err := fasthttp.Do(req, resp); err != nil {
		var temp []byte
		return 500, temp, err
	}

	ReqBody := resp.Body()

	StatusCode := resp.StatusCode()

	return StatusCode, ReqBody, nil
}


// get 请求
func HttpGet(url string) ([]byte, error) {

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源

	// 默认是application/x-www-form-urlencoded
	req.Header.SetContentType("application/json")
	req.Header.SetMethod("GET")

	req.SetRequestURI(url)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源

	if err := fasthttp.Do(req, resp); err != nil {
		var temp []byte
		return temp, err
	}

	ReqBody := resp.Body()

	return []byte(string(ReqBody)), nil

}


// head 请求
func HttpHead(Url string) (fasthttp.ResponseHeader, error) {

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源

	// 默认是application/x-www-form-urlencoded
	req.Header.SetContentType("application/json")
	req.Header.SetMethod("HEAD")

	req.SetRequestURI(Url)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源

	if err := fasthttp.Do(req, resp); err != nil {
		var temp fasthttp.ResponseHeader
		return temp, err
	}

	return resp.Header, nil

}


// 根据url获取channel
func GetChannelByUrl(PreloadUrl string) string {

	u, err := url.Parse(PreloadUrl)
	if err != nil {
		return ""
	}
	var ChannelBuilder strings.Builder
	ChannelBuilder.WriteString(u.Scheme)
	ChannelBuilder.WriteString("://")
	ChannelBuilder.WriteString(u.Hostname())
	ChannelName := ChannelBuilder.String()

	return ChannelName
}
