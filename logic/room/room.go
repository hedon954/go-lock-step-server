package room

import (
	"sync"
	"time"

	"github.com/hedon954/go-lock-step-server/pkg/network"
)

const (
	HeatbeatFrequency = 30
	TickTimer         = time.Second / HeatbeatFrequency
	TimeoutTime       = time.Minute * 5
)

type packet struct {
	id  uint64
	msg network.Packet
}

// Room is the Battle Room
type Root struct {
	wg sync.WaitGroup

	roomID      uint64
	players     []uint64
	typeID      int32
	closeFlag   int32
	timeStamp   int64
	secretKey   string
	logicServer string

	exitChan chan struct{}
	msgQ     chan *packet
	inChan   chan *network.Conn
	outChan  chan *network.Conn

	gane *game.Game
}
