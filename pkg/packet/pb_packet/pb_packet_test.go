package pb_packet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hedon954/go-lock-step-server/pb"
	"google.golang.org/protobuf/proto"
)

func Test_SCPacket(t *testing.T) {

	var sID int32 = 19234333
	msg := &pb.C2S_InputMsg{
		Sid: proto.Int32(sID),
		X:   proto.Int32(10),
		Y:   proto.Int32(20),
	}

	raw, _ := proto.Marshal(msg)
	p := NewPacket(uint8(pb.ID_MSG_Input), msg)
	if p == nil {
		t.Fail()
		return
	}

	buff := p.Serialize()
	dataLen := binary.BigEndian.Uint16(buff[0:])
	if dataLen != uint16(len(raw)) {
		t.Errorf("dataLen != uint16(len(raw)")
		return
	}

	id := buff[DataLen]
	if p.id != id {
		t.Errorf("uint8(ID_C2S_Connect) != id[%d]", id)
		return
	}

	msg1 := &pb.C2S_InputMsg{}
	if err := proto.Unmarshal(buff[MinPacketLen:], msg1); err != nil {
		t.Error(err)
		return
	}

	if msg.GetSid() != msg1.GetSid() || msg.GetX() != msg1.GetX() || msg.GetY() != msg1.GetY() {
		t.Errorf("want: %v, got: %v", msg, msg1)
		return
	}
}

func Benchmark_SCPacket(b *testing.B) {
	var sID int32 = 19234333
	msg := &pb.C2S_InputMsg{
		Sid: proto.Int32(sID),
		X:   proto.Int32(10),
		Y:   proto.Int32(20),
	}
	for i := 0; i < b.N; i++ {
		NewPacket(uint8(pb.ID_MSG_Input), msg)
	}
}

func Test_Packet(t *testing.T) {
	var sID int32 = 19234333
	msg := &pb.C2S_InputMsg{
		Sid: proto.Int32(sID),
		X:   proto.Int32(10),
		Y:   proto.Int32(20000),
	}

	temp, _ := proto.Marshal(msg)

	p := &Packet{
		id:   uint8(pb.ID_MSG_Input),
		data: temp,
	}

	b := p.Serialize()

	r := strings.NewReader(string(b))
	proto := &MsgProtocol{}
	ret, err := proto.ReadPacket(r)
	if err != nil {
		t.Error(err)
		return
	}
	packet, _ := ret.(*Packet)
	if packet.GetMessageID() != p.id {
		t.Errorf("want: %v, got: %v", p.id, packet.GetMessageID())
		return
	}
	if len(packet.GetData()) != len(p.data) {
		t.Errorf("datalen, want: %v, got: %v", len(p.data), len(packet.GetData()))
		return
	}

	msg1 := &pb.C2S_InputMsg{}
	err = packet.UnmarshalPB(msg1)
	if err != nil {
		t.Error(err)
		return
	}

	if msg.GetSid() != msg1.GetSid() || msg.GetX() != msg1.GetX() || msg.GetY() != msg1.GetY() {
		t.Errorf("want: %v, got: %v", msg, msg1)
		return
	}
}

func Benchmark_Packet(b *testing.B) {
	var sID int32 = 19234333
	msg := &pb.C2S_InputMsg{
		Sid: proto.Int32(sID),
		X:   proto.Int32(10),
		Y:   proto.Int32(20000),
	}

	temp, _ := json.Marshal(msg)

	p := &Packet{
		id:   uint8(pb.ID_MSG_Input),
		data: temp,
	}

	buf := p.Serialize()

	proto := &MsgProtocol{}

	r := bytes.NewBuffer(nil)

	for i := 0; i < b.N; i++ {
		r.Write(buf)
		if _, err := proto.ReadPacket(r); nil != err {
			b.Error(err)
		}
	}
}
