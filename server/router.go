package server

import (
	"sync/atomic"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/hedon954/go-lock-step-server/pb"
	"github.com/hedon954/go-lock-step-server/pkg/network"
	"github.com/hedon954/go-lock-step-server/pkg/packet/pb_packet"
)

// TODO
func verifyToken(token string) string {
	return token
}

func (r *LockStepServer) OnConnect(conn *network.Conn) bool {
	count := atomic.AddInt64(&r.totalConn, 1)
	log4go.Debug("[router] OnConnect [%s] totalConn=%d", conn.GetRawConn().RemoteAddr().String(), count)
	// TODO: here you can check something,
	//  and if there is something wrong, you can return false
	return true
}

func (r *LockStepServer) OnMessage(conn *network.Conn, p network.Packet) bool {
	msg := p.(*pb_packet.Packet)
	log4go.Info("[router] OnMessage [%s] msg=[%d] len=[%d]", conn.GetRawConn().RemoteAddr().String(),
		msg.GetMessageID(), len(msg.GetData()))

	switch pb.ID(msg.GetMessageID()) {
	case pb.ID_MSG_Connect:
		rec := &pb.C2S_ConnectMsg{}
		if err := msg.UnmarshalPB(rec); err != nil {
			log4go.Error("[router] msg.Unmarshal error=[%s]", err.Error())
			return false
		}
		playerID := rec.GetPlayerID()
		batteleID := rec.GetBatteleID()
		token := rec.GetToken()

		ret := &pb.S2C_ConnectMsg{
			ErrorCode: pb.ERRORCODE_ERR_ok.Enum(),
		}

		room := r.roomMgr.GetRoom(batteleID)
		if room == nil {
			ret.ErrorCode = pb.ERRORCODE_ERR_NoRoom.Enum()
			conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			log4go.Error("[router] no room player=[%d] room=[%d] token=[%s]", playerID, batteleID, token)
			return true
		}

		if room.IsOver() {
			ret.ErrorCode = pb.ERRORCODE_ERR_RoomState.Enum()
			conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			log4go.Error("[router] room is over player=[%d] room==[%d] token=[%s]", playerID, batteleID, token)
			return true
		}

		if !room.HasPlayer(playerID) {
			ret.ErrorCode = pb.ERRORCODE_ERR_NoPlayer.Enum()
			conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			log4go.Error("[router] !room.HasPlayer(playerID) player=[%d] room==[%d] token=[%s]", playerID, batteleID,
				token)
			return true
		}

		if token != verifyToken(token) {
			ret.ErrorCode = pb.ERRORCODE_ERR_Token.Enum()
			conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			log4go.Error("[router] verifyToken failed player=[%d] room==[%d] token=[%s]", playerID, batteleID, token)
			return true
		}

		conn.PutExtraData(playerID)
		return room.OnConnect(conn)

	case pb.ID_MSG_Heartbeat:
		conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_Heartbeat), nil), time.Millisecond)
		return true

	case pb.ID_MSG_END:
		conn.AsyncWritePacket(pb_packet.NewPacket(uint8(pb.ID_MSG_END), msg.GetData()), time.Millisecond)
		return true
	}

	return false
}

func (r *LockStepServer) OnClose(conn *network.Conn) {
	count := atomic.AddInt64(&r.totalConn, -1)
	log4go.Info("[router] OnClose: total=%d", count)
}
