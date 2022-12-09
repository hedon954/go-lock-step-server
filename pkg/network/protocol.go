package network

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrDataLengthOutOfLimit = errors.New("the size of packet is larger than the limit")
)

type Packet interface {
	Serialize() []byte
}

type Protocol interface {
	ReadPacket(conn io.Reader) (Packet, error)
}

type DefaultPacket struct {
	buff []byte
}

func (dp *DefaultPacket) Serialize() []byte {
	return dp.buff
}

func NewDefaultPacket(buff []byte) *DefaultPacket {
	p := &DefaultPacket{}

	p.buff = make([]byte, 4+len(buff))
	binary.BigEndian.PutUint32(p.buff[0:4], uint32(len(buff)))
	copy(p.buff[4:], buff)

	return p
}

type DefaultProtocol struct {
}

func (dp *DefaultProtocol) ReadPacket(r io.Reader) (Packet, error) {
	var lengthBytes = make([]byte, 4)
	var length uint32

	// read length
	if _, err := io.ReadFull(r, lengthBytes); err != nil {
		return nil, err
	}
	if length = binary.BigEndian.Uint32(lengthBytes); length > 1024 {
		return nil, ErrDataLengthOutOfLimit
	}

	// read body
	buff := make([]byte, length)
	if _, err := io.ReadFull(r, buff); err != nil {
		return nil, err
	}

	return NewDefaultPacket(buff), nil
}
