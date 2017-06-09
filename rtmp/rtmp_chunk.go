package rtmp

import (
	//"fmt"
	"bytes"
	"errors"

	"github.com/sevenzoe/gortmp/util"
)

// RTMP协议中基本的数据单元称为消息(Message).
// 当RTMP协议在互联网中传输数据的时候,消息会被拆分成更小的单元,称为消息块(Chunk).
// 在网络上传输数据时,消息需要被拆分成较小的数据块,才适合在相应的网络环境上传输.

// 理论上Type 0, 1, 2的Chunk都可以使用Extended Timestamp来传递时间
// Type 3由于严禁携带Extened Timestamp字段.但实际上只有Type 0才需要带此字段.
// 这是因为,对Type 1, 2来说,其时间为一个差值,一般肯定小于0x00FFFFF

// 对于除Audio,Video以外的基它Message,其时间字段都可以是置为0的，似乎没有被用到.
// 只有在发送视频和音频数据时,才需要特别的考虑TimeStamp字段.基本依据是,要以HandShake时为起始点0来计算时间.
// 一般来说,建立一个相对时间,把一个视频帧的TimeStamp特意的在当前时间的基础上延迟3秒,则可以达到缓存的效果

const (
	RTMP_CHUNK_HEAD_12 = 0 << 6 // Chunk Basic Header = (Chunk Type << 6) | Chunk Stream ID.
	RTMP_CHUNK_HEAD_8  = 1 << 6
	RTMP_CHUNK_HEAD_4  = 2 << 6
	RTMP_CHUNK_HEAD_1  = 3 << 6
)

type Chunk struct {
	Header ChunkHeader
	Body   ChunkBody
}

type ChunkBody struct {
	Payload []byte
}

type ChunkHeader struct {
	ChunkBasicHeader
	ChunkMessgaeHeader
	ChunkExtendedTimestamp
}

// Basic Header (1 to 3 bytes) : This field encodes the chunk stream ID
// and the chunk type. Chunk type determines the format of the
// encoded message header. The length(Basic Header) depends entirely on the chunk
// stream ID, which is a variable-length field.
type ChunkBasicHeader struct {
	ChunkStreamID uint32 `json:""` // 6 bit. 3 ~ 65559, 0,1,2 reserved
	ChunkType     byte   `json:""` // 2 bit.
}

// Message Header (0, 3, 7, or 11 bytes): This field encodes
// information about the message being sent (whether in whole or in
// part). The length can be determined using the chunk type
// specified in the chunk header.
type ChunkMessgaeHeader struct {
	Timestamp       uint32 `json:""` // 3 byte
	MessageLength   uint32 `json:""` // 3 byte
	MessageTypeID   byte   `json:""` // 1 byte
	MessageStreamID uint32 `json:""` // 4 byte
}

// Extended Timestamp (0 or 4 bytes): This field is present in certain
// circumstances depending on the encoded timestamp or timestamp
// delta field in the Chunk Message header. See Section 5.3.1.3 for
// more information
type ChunkExtendedTimestamp struct {
	ExtendTimestamp uint32 `json:",omitempty"` // 标识该字段的数据可忽略
}

// ChunkBasicHeader会决定ChunkMessgaeHeader,ChunkMessgaeHeader有4种(0,3,7,11 Bytes),因此可能有4种头.

// 1  -> ChunkBasicHeader(1) + ChunkMessgaeHeader(0)
// 4  -> ChunkBasicHeader(1) + ChunkMessgaeHeader(3)
// 8  -> ChunkBasicHeader(1) + ChunkMessgaeHeader(7)
// 12 -> ChunkBasicHeader(1) + ChunkMessgaeHeader(11)

func encodeChunk12(head *RtmpHeader, payload []byte, size int) (mark []byte, need []byte, err error) {
	if size > RTMP_MAX_CHUNK_SIZE || payload == nil || len(payload) == 0 {
		return nil, nil, errors.New("chunk error")
	}

	buf := new(bytes.Buffer)

	chunkBasicHead := byte(RTMP_CHUNK_HEAD_12 + head.ChunkBasicHeader.ChunkStreamID)
	buf.WriteByte(chunkBasicHead)

	b := make([]byte, 3)
	util.BigEndian.PutUint24(b, head.ChunkMessgaeHeader.Timestamp)
	buf.Write(b)

	util.BigEndian.PutUint24(b, head.ChunkMessgaeHeader.MessageLength)
	buf.Write(b)

	buf.WriteByte(head.ChunkMessgaeHeader.MessageTypeID)

	b = make([]byte, 4)
	util.LittleEndian.PutUint32(b, uint32(head.ChunkMessgaeHeader.MessageStreamID))
	buf.Write(b)

	if head.ChunkMessgaeHeader.Timestamp == 0xffffff {
		b := make([]byte, 4)
		util.LittleEndian.PutUint32(b, head.ChunkExtendedTimestamp.ExtendTimestamp)
		buf.Write(b)
	}

	if len(payload) > size {
		buf.Write(payload[0:size])
		need = payload[size:]
	} else {
		buf.Write(payload)
	}

	mark = buf.Bytes()

	return
}

func encodeChunk8(head *RtmpHeader, payload []byte, size int) (mark []byte, need []byte, err error) {
	if size > RTMP_MAX_CHUNK_SIZE || payload == nil || len(payload) == 0 {
		return nil, nil, errors.New("chunk error")
	}

	buf := new(bytes.Buffer)

	chunkBasicHead := byte(RTMP_CHUNK_HEAD_8 + head.ChunkBasicHeader.ChunkStreamID)
	buf.WriteByte(chunkBasicHead)

	b := make([]byte, 3)
	util.BigEndian.PutUint24(b, head.ChunkMessgaeHeader.Timestamp)
	buf.Write(b)

	util.BigEndian.PutUint24(b, head.ChunkMessgaeHeader.MessageLength)
	buf.Write(b)

	buf.WriteByte(head.ChunkMessgaeHeader.MessageTypeID)

	if len(payload) > size {
		buf.Write(payload[0:size])
		need = payload[size:]
	} else {
		buf.Write(payload)
	}

	mark = buf.Bytes()

	return
}

func encodeChunk4(head *RtmpHeader, payload []byte, size int) (mark []byte, need []byte, err error) {
	if size > RTMP_MAX_CHUNK_SIZE || payload == nil || len(payload) == 0 {
		return nil, nil, errors.New("chunk error")
	}

	buf := new(bytes.Buffer)

	chunkBasicHead := byte(RTMP_CHUNK_HEAD_4 + head.ChunkBasicHeader.ChunkStreamID)
	buf.WriteByte(chunkBasicHead)

	b := make([]byte, 3)
	util.BigEndian.PutUint24(b, head.ChunkMessgaeHeader.Timestamp)
	buf.Write(b)

	if len(payload) > size {
		buf.Write(payload[0:size])
		need = payload[size:]
	} else {
		buf.Write(payload)
	}

	mark = buf.Bytes()

	return
}

func encodeChunk1(head *RtmpHeader, payload []byte, size int) (mark []byte, need []byte, err error) {
	if size > RTMP_MAX_CHUNK_SIZE || payload == nil || len(payload) == 0 {
		return nil, nil, errors.New("chunk error")
	}

	buf := new(bytes.Buffer)

	chunkBasicHead := byte(RTMP_CHUNK_HEAD_1 + head.ChunkBasicHeader.ChunkStreamID)
	buf.WriteByte(chunkBasicHead)

	if len(payload) > size {
		buf.Write(payload[0:size])
		need = payload[size:]
	} else {
		buf.Write(payload)
	}

	mark = buf.Bytes()

	return
}
