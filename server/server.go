package server

import (
	"github.com/hedon954/go-lock-step-server/logic"
	"github.com/hedon954/go-lock-step-server/pkg/kcp_server"
	"github.com/hedon954/go-lock-step-server/pkg/network"
	"github.com/hedon954/go-lock-step-server/pkg/packet/pb_packet"
)

// LockStepServer is a lock step server
type LockStepServer struct {
	roomMgr   *logic.RoomManager
	udpServer *network.Server
	totalConn int64
}

// New creates a new lock step server
func New(address string) (*LockStepServer, error) {
	s := &LockStepServer{
		roomMgr: logic.NewRoomManager(),
	}
	networkServer, err := kcp_server.ListenAndServe(address, s, &pb_packet.MsgProtocol{})
	if err != nil {
		return nil, err
	}
	s.udpServer = networkServer
	return s, nil
}

//  RoomManager gets room manager
func (r *LockStepServer) RoomManager() *logic.RoomManager {
	return r.roomMgr
}

// Stop stops the server
func (r *LockStepServer) Stop() {
	r.roomMgr.Stop()
	r.udpServer.Stop()
}
