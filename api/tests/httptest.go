package tests

import (
	"bytes"
	"io/ioutil"
	"time"

	//"bytes"
	//"encoding/json"
	//"io/ioutil"
	//"net/http"
	//"net/http/httptest"
	//"testing"
	//"runtime"
	//"path/filepath"
	////_ "api/routers"
	//"time"
	//
	"encoding/json"
	"github.com/astaxie/beego"
	"net/http"

	//. "github.com/smartystreets/goconvey/convey"

)

//func init() {
//	_, file, _, _ := runtime.Caller(1)
//	apppath, _ := filepath.Abs(filepath.Dir(filepath.Join(file, ".." + string(filepath.Separator))))
//	beego.TestBeegoInit(apppath)
//}

// TestGet is a sample to run an endpoint test
//func TestGet(t *testing.T) {
//	r, _ := http.NewRequest("GET", "/v1/object", nil)
//	w := httptest.NewRecorder()
//	beego.BeeApp.Handlers.ServeHTTP(w, r)
//
//	beego.Trace("testing", "TestGet", "Code[%d]\n%s", w.Code, w.Body.String())
//
//	Convey("Subject: Test Station Endpoint\n", t, func() {
//	        Convey("Status Code Should Be 200", func() {
//	                So(w.Code, ShouldEqual, 200)
//	        })
//	        Convey("The Result Should Not Be Empty", func() {
//	                So(w.Body.Len(), ShouldBeGreaterThan, 0)
//	        })
//	})
//}

func TestPost(url string, data interface{}, contentType string) string {
	beego.Debug(1111111111111)
	beego.Debug(data)
	beego.Debug(222222222222)
	jsonStr, _ := json.Marshal(data)
	req, err := http.NewRequest(`POST`, url, bytes.NewBuffer(jsonStr))
	req.Header.Add(`content-type`, contentType)
	if err != nil {
		panic(err)
	}
	defer req.Body.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	result, _ := ioutil.ReadAll(resp.Body)
	return string(result)
}