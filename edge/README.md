# 设置代理 

> export GOPROXY=https://goproxy.io 

# 调试  

> go run main.go preload 

# 编译 

> go build -ldflags "-w -s -X \"edge/cmd.BUILD_TIME=`date '+%Y-%m-%d %H:%M:%S'`\" -X \"edge/cmd.GO_VERSION=`go version`\"" -o preloader 

# 项目结构  

cmd/root.go  固定生成，要维护版本NextVersion 

cmd/preload.go 主要业务代码，采用gin框架 

common/utils.go 公用代码，日志代码

# 功能 

1. /debug/pprof/ 
 
性能监控 

2. /ping 

test 

3. /edge/preload/task 

边缘接收任务接口 

4. /edge/preload/task/cancel 

边缘任务中断接口 

5. ./preloader -v 

查看版本

# cobra初始化 
只有在重新加模块时使用，具体使用方法，见官方文档，会在GOPATH下生成项目，需要移到指定路径

> cobra init edge 

> cobra add preload 
