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
	CSID uint32 // 6 bit. 3 ~ 65559, 0,1,2 reserved
	FMT  byte   // 2 bit.
	Data []byte // net frame data
	refs int32  // 回收计数，当计数为0，则自动回收至池中
}

func (nf *NetFrame) Refs(refs int32) int32 {
	refs = atomic.AddInt32(&nf.refs, refs)
	if refs == 0 {
		PutFrame(nf)
	}
	return refs
}

func (nf *NetFrame) Assign(baseHead *ChunkBasicHeader) {
	nf.FMT = baseHead.ChunkType
	nf.CSID = baseHead.ChunkStreamID
}

func (nf *NetFrame) Append(b byte) {
	nf.Data = append(nf.Data, b)
}

func (nf *NetFrame) Appends(bs []byte) {
	nf.Data = append(nf.Data, bs...)
}

var (
	NetFrameSize = 512
	netFramePool = sync.Pool{New: func() interface{} { return &NetFrame{Data: make([]byte, 0, NetFrameSize)} }}
)

func GetFrame() *NetFrame {
	frame := netFramePool.Get().(*NetFrame)
	frame.Data = frame.Data[:0]
	return frame
}

func PutFrame(frame *NetFrame) {
	frame.Data = frame.Data[:0]
	netFramePool.Put(frame)
}
