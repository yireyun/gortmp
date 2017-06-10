// webSocketPool.go
package main

import (
	"fmt"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/sevenzoe/gortmp/rtmp"
)

const (
	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type WsStat struct {
	wsc       *ws.Conn
	wscAddr   string
	frameC    chan *rtmp.NetFrame
	isClosed  bool
	isFMT0    bool //表示首帧以推送
	sendTimes int
	sendSize  int
	recvTimes int
	recvSize  int
}

func NewStat(wsc *ws.Conn) *WsStat {
	return &WsStat{
		wsc:     wsc,
		wscAddr: wsc.RemoteAddr().String(),
		frameC:  make(chan *rtmp.NetFrame, 64),
	}
}

func (c *WsStat) recvLoop(removeC chan *ws.Conn) {
	defer func() {
		if x := recover(); x != nil {
			fmt.Printf("WebSocket [%v] RecvLoop Error: %v\n", c.wscAddr, x)
		}
	}()

	for !c.isClosed {
		msgType, msgData, err := c.wsc.ReadMessage()
		if err != nil {
			fmt.Printf("WebSocket [%v] RecvLoop Error: %v\n", c.wscAddr, err)
			c.isClosed = true
		} else {
			fmt.Printf("WebSocket [%v] RecvLoop: {msgType:%v, msgData:[%x]}\n", c.wscAddr, msgType, msgData)
		}

	}
}

func (c *WsStat) pushLoop(removeC chan *ws.Conn) {
	defer func() {
		if x := recover(); x != nil {
			fmt.Printf("WebSocket [%v] PushLoop Error: %v\n", c.wscAddr, x)
		}
	}()

	pingTicker := time.NewTicker(pingPeriod)
	for !c.isClosed {
		select {
		case <-pingTicker.C:
			c.wsc.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.wsc.WriteMessage(ws.PingMessage, []byte{}); err != nil {
				fmt.Printf("WebSocket [%v] PingMessage Error: %v\n", c.wscAddr, err)
			} else {
				fmt.Printf("WebSocket [%v] PingMessage Success\n", c.wscAddr)
			}
		case frame, ok := <-c.frameC:
			if !ok {
				fmt.Printf("WebSocket [%v] Conn Close\n", c.wscAddr)
				c.wsc.Close()
				c.isClosed = true
			} else {
				if !c.isFMT0 && frame.FMT != 0 {
					dateLen := int32(len(frame.Data))
					fmt.Printf("<%v>:{FMT: %v, CSID: %v}, "+
						"{Timestamp: %6v, MsgLength: %4v, MsgTypeID: %v, StreamID: %v, ExTimestamp: %v}, "+
						"{DataLen: %4v, HeadLen: %4v, MissLen: %4v}\n",
						c.sendTimes, frame.FMT, frame.CSID, frame.Timestamp,
						frame.MsgLength, frame.MsgTypeID, frame.StreamID,
						frame.ExTimestamp, dateLen, frame.HeadLength, dateLen-int32(frame.HeadLength)-int32(frame.MsgLength))
				}
				if frame != nil && (c.isFMT0 || frame.FMT == 0) {
					c.isFMT0 = true
					c.wsc.SetWriteDeadline(time.Now().Add(writeWait))
					err := c.wsc.WriteMessage(ws.BinaryMessage, frame.Data)
					if err != nil {
						fmt.Printf("WebSocket [%v] Push Frame Error: %v\n", c.wscAddr, err)
						c.wsc.Close()
						c.isClosed = true
					} else {
						c.sendTimes++
						c.sendSize += len(frame.Data)
						//if c.sendTimes%5 == 0 {
						//	fmt.Printf("<%v>:{FMT: %v, CSID: %v, Size: %v}\n",
						//	c.sendTimes, frame.FMT, frame.CSID, len(frame.Data))
						//} else {
						//	fmt.Printf("<%v>:{FMT: %v, CSID: %v, Size: %v}\t",
						//	c.sendTimes, frame.FMT, frame.CSID, len(frame.Data))
						//}
						if c.sendTimes%100 == 0 {
							fmt.Printf("<%v>:{FMT: %v, CSID: %v, Size: %v}\n",
								c.sendTimes, frame.FMT, frame.CSID, c.sendSize)
						}
					}
				}
			}
			if frame != nil {
				frame.Refs(-1)
			}
		}
	}
	for {
		select {
		case frame := <-c.frameC:
			if frame != nil {
				frame.Refs(-1)
			}
		}
	}
}

func (c *WsStat) RecvLoop(removeC chan *ws.Conn) {
	for !c.isClosed {
		c.recvLoop(removeC)
	}
	removeC <- c.wsc
}

func (c *WsStat) PushLoop(removeC chan *ws.Conn) {
	for !c.isClosed {
		c.pushLoop(removeC)
	}
	removeC <- c.wsc
}

type WsPool struct {
	wscm     map[*ws.Conn]*WsStat
	appendC  chan *ws.Conn
	removeC  chan *ws.Conn
	frameC   chan *rtmp.NetFrame
	stopC    chan struct{}
	isStop   bool
	frameCnt int
	mu       sync.RWMutex
}

func NewWsPool() *WsPool {
	return &WsPool{
		wscm:    make(map[*ws.Conn]*WsStat),
		appendC: make(chan *ws.Conn, 16),
		removeC: make(chan *ws.Conn, 16),
		frameC:  make(chan *rtmp.NetFrame, 64),
		stopC:   make(chan struct{}, 2),
	}
}

func (p *WsPool) doFor() {
	defer func() {
		if x := recover(); x != nil {
			fmt.Printf("WebSocket Process Error: %v\n", x)
		}
	}()

	for {
		select {
		case wsc, ok := <-p.appendC:
			if wsc != nil && ok {
				p.mu.Lock()
				_, exist := p.wscm[wsc]
				if !exist {
					stat := NewStat(wsc)
					p.wscm[wsc] = stat
					go stat.PushLoop(p.removeC)
					go stat.RecvLoop(p.removeC)
				}
				p.mu.Unlock()
				fmt.Printf("Append WebSocket Conn: %v\n", wsc.RemoteAddr().String())
			}
		case wsc, ok := <-p.removeC:
			if wsc != nil && ok {
				p.mu.Lock()
				stat, exist := p.wscm[wsc]
				if exist {
					delete(p.wscm, wsc)
				}
				if !stat.isClosed {
					stat.wsc.Close()
				}
				p.mu.Unlock()
				fmt.Printf("Remove WebSocket Conn: %v, %+v\n", wsc.RemoteAddr().String(), stat)
			}
		case frame, ok := <-p.frameC:
			if frame != nil && ok {
				p.frameCnt++
				//msg := fmt.Sprintf("<%v>:{FMT: %v, CSID: %v, Size: %v}",
				//p.frameCnt, frame.FMT, frame.CSID, len(frame.Data))
				pushCnt := 0
				p.mu.Lock()
				for _, stat := range p.wscm {
					if !stat.isClosed {
						stat.frameC <- frame
						frame.Refs(1)
						pushCnt++
					}
				}
				p.mu.Unlock()
				//fmt.Printf("%s, pushCnt： %v\n", msg, pushCnt)
				frame.Refs(0)
			}
		case _, ok := <-p.stopC:
			if !ok {
				p.isStop = true
				return
			}
		}
	}
}

func (p *WsPool) Run() {
	p.isStop = false
	for !p.isStop {
		p.doFor()
	}
}

func (p *WsPool) Stop() {
}

func (p *WsPool) Append(wsc *ws.Conn) {
	p.appendC <- wsc
}

func (p *WsPool) Remove(wsc *ws.Conn) {
	p.removeC <- wsc
}

func (p *WsPool) PushRtmpNetFrame(frame *rtmp.NetFrame) {
	p.frameC <- frame
}

func (p *WsPool) WsCount() int {
	p.mu.Lock()
	cnt := len(p.wscm)
	p.mu.Unlock()
	return cnt
}
