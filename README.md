# 一、开发环境部署 
## 1.git clone 创建 
### 拉取代码 

> git@223.202.202.47:fusion/preloader.git 

### 设置代理

由于有些库不兼容，项目使用go1.12

go1.12

> export GOPROXY=https://goproxy.io 

go1.13

> go env -w GOPROXY=https://goproxy.io,direct 

### 运行项目 
> go run main.go

### 测试 

``` 
curl -X POST -i 'http://127.0.0.1:8080/preload/receive' --data '{
	"username":"365noc",
    "tasks": [{
            "url": "http://yqad.365noc.com:8080/FTP/FTP_D/yx-8-liyangpingling/babaiban20181015/babaiban20181015_3D/0b1d5cb0-1782-42a8-bf08-ccd2e66d40f9_cpl.xml",
            "validationType": "md5",  
			"validation": "9bd5e831444e2443fa37b231b01556a0"
        },{
            "url": "http://yqad.365noc.com/FTP/FTP_D/yx-8-liyangpingling/babaiban20181015/babaiban20181015_3D/0b1d5cb0-1782-42a8-bf08-ccd2e66d40f9.xml",
            "validationType": "basic",   
			"validation": "12"
        }
    ],
	"runTime": "2018-10-16 17:25:00",  
	"bandwidth": "100k",
    "compressed": 1,
	"layer":1 ,
    "header":"Referer: https://developer.mozilla.org/en-US/docs/Web/JavaScript"	
}'
```
## 2.从零开始本地创建 
### 创建目录
> mkdir preloader 
> cd preloader 
### 初始化 
> go mod init preloader 
### 设置代理
> go env -w GOPROXY=https://goproxy.io,direct 

### 安装beego 
> go get -u github.com/astaxie/beego 

### 安装 bee
> GO111MODULE=off go get -u 
github.com/beego/bee 

> GO111MODULE=on 

### 创建api项目 
> bee api api

### 默认安装在gopath中，需要迁移到指定目录内
> mv $GOPATH/src/api . 

> cd api

### 要运行项目，还需要再创建一遍运行环境，不能直接用bee run
> go mod init api 

> go run main.go# preloader
