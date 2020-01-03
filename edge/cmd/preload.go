// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	// "net"
	// "github.com/spf13/cobra"
	// "crypto/tls"
	comm "edge/common"
	"github.com/spf13/viper"
	ginlogrus "github.com/toorop/gin-logrus" // v0.0.0-20190701131413-6c374ad36b67
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var Log = comm.Log
var TaskMap taskMap

// type httpMap struct {
// 	httpMap map[string]*net.TCPConn
// }

// func (m httpMap) close(k string) {
// 	conn, ok := m[k]
// 	if ok {
// 		m[k].Close()
// 	}
// }

// type httpsMap struct {
// 	httpsMap map[string]*tls.Conn
// }

// func (m httpsMap) close(k string) {
// 	conn, ok := m[k]
// 	if ok {
// 		m[k].Close()
// 	}
// }

// var httpMap map[string]*net.TCPConn
// var httpsMap map[string]*tls.Conn

var (
	ipTo                                                                     string
	sslIpTo                                                                  string
	readTimeout                                                              int
	MaxConcurrent                                                            int
	TsConcurrent                                                             int
	ConnConcurrent                                                           int
	RangeConcurrent                                                          int
	PartialSize                                                              int
	MaxConcurrent_ch, TsConcurrent_ch, ConnConcurrent_ch, RangeConcurrent_ch chan int
)

func newConfig() {
	ipTo = viper.GetString("ip_to")
	sslIpTo = viper.GetString("ssl_ip_to")
	readTimeout = viper.GetInt("read_timeout")
	MaxConcurrent = viper.GetInt("maxConcurrent")
	TsConcurrent = viper.GetInt("TsConcurrent")
	ConnConcurrent = viper.GetInt("ConnConcurrent")
	RangeConcurrent = viper.GetInt("RangeConcurrent")
	PartialSize = viper.GetInt("PartialSize")

	MaxConcurrent_ch = make(chan int, MaxConcurrent)
	TsConcurrent_ch = make(chan int, TsConcurrent)
	ConnConcurrent_ch = make(chan int, ConnConcurrent)
	RangeConcurrent_ch = make(chan int, RangeConcurrent)
}

// @title preload Agent API
// @version 1.0.0
// @description  ???agent??API??.
func server() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginlogrus.Logger(Log))
	router.Use(gin.Recovery())

	//test
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// tasks to goPreload
	router.POST("/", TaskRequestPost)
	// 边缘接收任务接口
	router.POST("/edge/preload/task", task)
	router.POST("/edge/preload/task/cancel", cancel)
	router.POST("/edge/preload/task/check", check)

	//??????
	g := router.Group("/debug/pprof")
	{
		g.GET("/", gin.WrapF(pprof.Index))
		g.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		g.GET("/profile", gin.WrapF(pprof.Profile)) // cpu profile
		g.GET("/symbol", gin.WrapF(pprof.Symbol))
		g.GET("/trace", gin.WrapF(pprof.Trace))
		g.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))             //
		g.GET("/block", gin.WrapH(pprof.Handler("block")))               //
		g.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))               //
		g.GET("/heap", gin.WrapH(pprof.Handler("heap")))                 // heap allocation
		g.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))       // current goroutine
		g.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate"))) // os thread create
	}

	port := fmt.Sprintf(":%d", viper.GetInt("port"))
	srv := &http.Server{
		Addr:    port,
		Handler: router,
	}

	go func() {
		// ?????
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Log.Fatalf("listen: %s\n", err)
		}
	}()
	// ???????????????????
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	Log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		Log.Fatal("Server Shutdown:", err)
	}
	Log.Println("Server exiting")

}

// preloadCmd represents the preload command
var preloadCmd = &cobra.Command{
	Use:   "preload",
	Short: "preload agent",
	Long:  `?????`,
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("preload called")
		server()
	},
}

func init() {
	rootCmd.AddCommand(preloadCmd)
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	cpu := viper.GetInt("cpunum")
	if cpu > 0 {
		runtime.GOMAXPROCS(cpu)
	}
	level := viper.GetString("loglevel")

	Log.SetLevel(comm.LogLevel(level))

	// trace the tasks
	TaskMap = taskMap{}
	TaskMap.tMap = map[string]interface{}{}
	// TaskMap.newTaskMap()
	// httpMap = make(map[string]*net.TCPConn)
	// httpsMap = make(map[string]*tls.Conn)
	newConfig()
	// for goPreload's configuration
	Log.Infof("init cap(MaxConcurrent_ch): %v|| len(MaxConcurrent_ch): %v|| MaxConcurrent: %v|| ipTo: %v|| sslIpTo: %v|| readTimeout: %v|| TsConcurrent: %v|| ConnConcurrent: %v|| RangeConcurrent: %v|| PartialSize: %v|| cpu: %v", cap(MaxConcurrent_ch), len(MaxConcurrent_ch), MaxConcurrent, ipTo, sslIpTo, readTimeout, TsConcurrent, ConnConcurrent, RangeConcurrent, PartialSize, cpu)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// preloadCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// preloadCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
