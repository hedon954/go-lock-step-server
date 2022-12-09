package pb_packet

import (
	"encoding/binary"
	"io"

	"github.com/alecthomas/log4go"
	"github.com/hedon954/go-lock-step-server/pkg/network"
	"google.golang.org/protobuf/proto"
)

const (
	DataLen      = 2
	MessageIDLen = 1

	MinPacketLen = DataLen + MessageIDLen
	MaxPacketLen = (2 << 8) * DataLen
	MaxMessageID = (2 << 8) * MessageIDLen
)

/*

s -> c

|--totalDataLen(uint16)--|--msgIDLen(uint8)--|--------------data--------------|
|-------------2----------|---------1---------|---------(totalDataLen-2-1)-----|

*/

// Packet is the message sent by the server to the client
type Packet struct {
	id   uint8
	data []byte
}

func (p *Packet) GetMessageID() uint8 {
	return p.id
}

func (p *Packet) GetData() []byte {
	return p.data
}

func (p *Packet) Serialize() []byte {
	buff := make([]byte, MinPacketLen, MinPacketLen)

	// set field `totalDataLen`
	dataLen := len(p.data)
	binary.BigEndian.PutUint16(buff, uint16(dataLen))

	// set field `msgIDLen`
	buff[DataLen] = p.id

	// set field `data`
	return append(buff, p.data...)
}

func (p *Packet) UnmarshalPB(msg proto.Message) error {
	return proto.Unmarshal(p.data, msg)
}

// NewPacket creates a new packet
func NewPacket(id uint8, msg interface{}) *Packet {

	p := &Packet{
		id: id,
	}

	switch v := msg.(type) {
	case []byte:
		p.data = v
	case proto.Message:
		mdata, err := proto.Marshal(v)
		if err != nil {
			log4go.Error("[NewPacket] proto marshal msg: %d error: %v", id, err)
			return nil
		}
		p.data = mdata
	case nil:
	default:
		log4go.Error("[NewPacket] type unsupported: %d", id)
		return nil
	}

	return p
}

// MsgProtocol is used to read message according to protocol
type MsgProtocol struct {
}

func (p *MsgProtocol) ReadPacket(r io.Reader) (network.Packet, error) {
	buff := make([]byte, MinPacketLen, MinPacketLen)

	// read data length
	if _, err := io.ReadFull(r, buff); err != nil {
		return nil, err
	}
	dataLen := binary.BigEndian.Uint16(buff)
	if dataLen > MaxPacketLen {
		return nil, network.ErrDataLengthOutOfLimit
	}

	// set id
	msg := &Packet{
		id: buff[DataLen],
	}

	// read data
	if dataLen > 0 {
		msg.data = make([]byte, dataLen, dataLen)
		if _, err := io.ReadFull(r, msg.data); err != nil {
			return nil, err
		}
	}

	return msg, nil
}
