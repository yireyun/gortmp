// rtmp_netframe
package rtmp

import (
	"sync"
	"sync/atomic"
)

type NetFramer interface {
	OnRecvFrame(frame *NetFrame)
}

type NetFrame struct {
	CSID        uint32 // 6 bit. 3 ~ 65559, 0,1,2 reserved
	FMT         byte   // 2 bit.
	Timestamp   uint32 //
	MsgLength   uint32 //
	MsgTypeID   byte   //
	StreamID    uint32 //
	ExTimestamp uint32 //
	HeadLength  int32  //
	Data        []byte // net frame data
	refs        int32  // 回收计数，当计数为0，则自动回收至池中
}

func (nf *NetFrame) Refs(refs int32) int32 {
	refs = atomic.AddInt32(&nf.refs, refs)
	if refs == 0 {
		PutFrame(nf)
	}
	return refs
}

func (nf *NetFrame) Assign(header *RtmpHeader) {
	nf.FMT = header.ChunkType
	nf.CSID = header.ChunkStreamID
	nf.Timestamp = header.Timestamp
	nf.MsgLength = header.MessageLength
	nf.MsgTypeID = header.MessageTypeID
	nf.StreamID = header.MessageStreamID
	nf.ExTimestamp = header.ExtendTimestamp
}

func (nf *NetFrame) Append(b byte, head int32) {
	nf.Data = append(nf.Data, b)
	nf.HeadLength += head
}

func (nf *NetFrame) Appends(bs []byte, head int32) {
	nf.Data = append(nf.Data, bs...)
	nf.HeadLength += head
}

var (
	NetFrameSize = 512
	netFramePool = sync.Pool{New: func() interface{} { return &NetFrame{Data: make([]byte, 0, NetFrameSize)} }}
)

func GetFrame() *NetFrame {
	frame := netFramePool.Get().(*NetFrame)
	frame.Data = frame.Data[:0]
	frame.HeadLength = 0
	return frame
}

func PutFrame(frame *NetFrame) {
	frame.Data = frame.Data[:0]
	frame.HeadLength = 0
	netFramePool.Put(frame)
}
