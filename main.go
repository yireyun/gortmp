package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	//"fmt"
	//"os"
	"net/http"
	"sync"
	"time"

	"github.com/sevenzoe/gortmp/config"
	"github.com/sevenzoe/gortmp/rtmp"
	//"github.com/sevenzoe/gortmp/avformat"
	//"github.com/sevenzoe/gortmp/mpegts"
)

var wsPool = NewWsPool()

type ServerHandler struct {
	frameCnt int
	rtmp.DefaultServerHandler
}

func (p *ServerHandler) OnRecvFrame(frame *rtmp.NetFrame) {
	p.frameCnt++
	//收到网络数据帧事件
	//fmt.Printf("<%v>:{FMT: %v, CSID: %v, Size: %v}\n", p.frameCnt, frame.FMT, frame.CSID, len(frame.Data))
	wsPool.PushRtmpNetFrame(frame)
}

var handler rtmp.ServerHandler = &ServerHandler{}

func ListenAndServe(addr string) error {
	s := rtmp.Server{
		Addr:        addr,                            // 服务器的IP地址和端口信息
		Handler:     handler,                         // 请求处理函数的路由复用器
		ReadTimeout: time.Duration(time.Second * 15), // timeout
		WriteTimout: time.Duration(time.Second * 15), // timeout
		Lock:        new(sync.Mutex)}                 // lock
	return s.ListenAndServer()
}

func main() {
	var err error

	if err = InitAppConfig(); err != nil {
		return
	}

	go wsPool.Run()

	l := ":1935"
	err = ListenAndServe(l)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		panic(err)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	sig := <-ch
	fmt.Printf("Signal received: %v\n", sig)

}

func InitAppConfig() (err error) {
	cfg := new(config.Config)
	err = cfg.Init("app.conf")
	if err != nil {
		return
	}

	return
}
