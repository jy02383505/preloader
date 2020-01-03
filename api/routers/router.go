// @APIVersion 1.0.0
// @Title beego Test API
// @Description beego has a very cool tools to autogenerate documents for your API
// @Contact astaxie@gmail.com
// @TermsOfServiceUrl http://beego.me/
// @License Apache 2.0
// @LicenseUrl http://www.apache.org/licenses/LICENSE-2.0.html
package routers

import (
	"api/controllers"

	"github.com/astaxie/beego"
)

func init() {
	// 接收任务
	beego.Router("/fusion/preload/receive", &controllers.ReceiveController{}, "*:Receive")

	// 任务上报
	beego.Router("/fusion/preload/report", &controllers.ReportController{}, "*:Report")

	// 任务查询
	beego.Router("/fusion/preload/query", &controllers.QueryController{}, "*:Query")

	// 任务中断
	beego.Router("/fusion/preload/cancel", &controllers.CancelController{}, "*:Cancel")

	// 任务重试
	beego.Router("/fusion/preload/retry", &controllers.RetryController{}, "*:Retry")

	// 测试
	beego.Router("/fusion/preload/temp", &controllers.TempController{}, "*:Temp")
}
