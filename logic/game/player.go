package game

import (
	"time"

	"github.com/hedon954/go-lock-step-server/pkg/network"
)

// Player defines a player state
type Player struct {
	id                uint64
	idx               int32
	isReady           bool
	isOnline          bool
	loadingProgress   int32
	lastHeartbeatTime int64
	sendFrameCount    uint32
	client            *network.Conn
}

// NewPlayer creates a new player state
func NewPlayer(id uint64, idx int32) *Player {
	return &Player{
		id:  id,
		idx: idx,
	}
}

func (p *Player) Connect(conn *network.Conn) {
	p.client = conn
	p.isOnline = true
	p.isReady = true
	p.lastHeartbeatTime = time.Now().Unix()
}

func (p *Player) IsOnline() bool {
	return p.client != nil && p.isOnline
}

func (p *Player) RefreshHeartbeat() {
	p.lastHeartbeatTime = time.Now().Unix()
}

func (p *Player) GetLastHeartbeatTime() int64 {
	return p.lastHeartbeatTime
}

func (p *Player) SetSendFrameCount(c uint32) {
	p.sendFrameCount = c
}

func (p *Player) GetSendFrameCount() uint32 {
	return p.sendFrameCount
}

func (p *Player) SendMessage(msg network.Packet) {
	if !p.isOnline {
		return
	}
	if p.client.AsyncWritePacket(msg, 0) != nil {
		p.client.Close()
	}
}

func (p *Player) Cleanup() {
	if p.client != nil {
		p.client.Close()
	}
	p.client = nil
	p.isOnline = false
	p.isReady = false
}
