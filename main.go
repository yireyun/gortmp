package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	//"fmt"
	//"os"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/sevenzoe/gortmp/config"
	"github.com/sevenzoe/gortmp/rtmp"
	//"github.com/sevenzoe/gortmp/avformat"
	//"github.com/sevenzoe/gortmp/mpegts"
)

const isTrac = false

var (
	tracRv *os.File
	wsPool = NewWsPool()
)

type ServerHandler struct {
	frameCnt int
	rtmp.DefaultServerHandler
}

func (p *ServerHandler) OnRecvFrame(frame *rtmp.NetFrame) {
	if isTrac {
		if p.frameCnt <= 100 {
			tracRv.Write([]byte(hex.EncodeToString(frame.Data)))
			tracRv.Write([]byte("\n"))
		}
	}

	p.frameCnt++

	//收到网络数据帧事件
	dateLen := int32(len(frame.Data))
	fmt.Printf("<%v>:{FMT: %v, CSID: %v}, "+
		"{Timestamp: %6v, MsgLength: %4v, MsgTypeID: %v, StreamID: %v, ExTimestamp: %v}, "+
		"{DataLen: %4v, HeadLen: %4v, MissLen: %4v}\n",
		p.frameCnt, frame.FMT, frame.CSID, frame.Timestamp,
		frame.MsgLength, frame.MsgTypeID, frame.StreamID,
		frame.ExTimestamp, dateLen, frame.HeadLength, dateLen-int32(frame.HeadLength)-int32(frame.MsgLength))

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

func pathJs(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir("./js")).ServeHTTP(w, r)
}

func main() {
	var err error

	if err = InitAppConfig(); err != nil {
		return
	}

	go wsPool.Run()

	err = ListenAndServe(":1935")
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.Handle("/js/", http.FileServer(http.Dir("./")))
	//	http.HandleFunc("/js/", pathJs)
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

func init() {
	if isTrac {
		tracRv, _ = os.Create("./tracRv.txt")
	}
}
