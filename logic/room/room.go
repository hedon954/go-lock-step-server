package room

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/hedon954/go-lock-step-server/logic/game"
	"github.com/hedon954/go-lock-step-server/pkg/network"
	"github.com/hedon954/go-lock-step-server/pkg/packet/pb_packet"
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
type Room struct {
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

	g *game.Game
}

// NewRoom creates a game room
func NewRoom(rid uint64, typeID int32, players []uint64, randomSeed int32, logicServer string) *Room {
	r := &Room{
		roomID:      rid,
		players:     players,
		typeID:      typeID,
		exitChan:    make(chan struct{}),
		msgQ:        make(chan *packet, 2048),
		outChan:     make(chan *network.Conn, 8),
		inChan:      make(chan *network.Conn, 8),
		timeStamp:   time.Now().Unix(),
		logicServer: logicServer,
		secretKey:   "test_room",
	}

	r.g = game.NewGame(rid, players, randomSeed, r)
	return r
}

func (r *Room) ID() uint64 {
	return r.roomID
}

func (r *Room) SecretKey() string {
	return r.secretKey
}

func (r *Room) TimeStamp() int64 {
	return r.timeStamp
}

func (r *Room) IsOver() bool {
	return atomic.LoadInt32(&r.closeFlag) != 0
}

func (r *Room) HasPlayer(id uint64) bool {
	for _, v := range r.players {
		if v == id {
			return true
		}
	}
	return false
}

func (r *Room) OnJoinGame(gid uint64, pid uint64) {
	log4go.Warn("[room(%d)] onJoinGame %d", gid, pid)
}

func (r *Room) OnGameStart(gid uint64) {
	log4go.Warn("[room(%d)] onGameStart", gid)
}

func (r *Room) OnLeaveGame(gid uint64, pid uint64) {
	log4go.Warn("[room(%d)] onLeaveGame %d", gid, pid)
}

func (r *Room) OnGameOver(gid uint64) {
	atomic.StoreInt32(&r.closeFlag, 1)
	log4go.Warn("[room(%d)] onGameOver", gid)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		// TODO
		// http result
	}()
}

// OnConnect network.Conn callback
func (r *Room) OnConnect(conn *network.Conn) bool {
	conn.SetCallback(r)
	r.inChan <- conn
	log4go.Warn("[room(%d)] OnConnect %d", r.roomID, conn.GetExtraData().(uint64))
	return true
}

// OnMessage network.Conn callback
func (r *Room) OnMessage(conn *network.Conn, msg network.Packet) bool {
	id, ok := conn.GetExtraData().(uint64)
	if !ok {
		log4go.Error("[room] OnMessage error conn don't have id")
		return false
	}

	p := &packet{
		id:  id,
		msg: msg,
	}
	r.msgQ <- p
	return true
}

// OnClose network.Conn callback
func (r *Room) OnClose(conn *network.Conn) {
	r.outChan <- conn
	id, ok := conn.GetExtraData().(uint64)
	if !ok {
		log4go.Warn("[room(%d)] OnClose no id", r.roomID)
		return
	}
	log4go.Warn("[room(%d)] OnClose %d", r.roomID, id)
}

// Stop force stop
func (r *Room) Stop() {
	close(r.exitChan)
	r.wg.Wait()
}

// Run is the main loop
func (r *Room) Run() {
	r.wg.Add(1)
	defer r.wg.Done()
	defer func() {
		r.g.Cleanup()
		log4go.Warn("[room(%d)] quit! total time=[%d]", r.roomID, time.Now().Unix()-r.timeStamp)
	}()

	tickerTick := time.NewTicker(TickTimer)
	defer tickerTick.Stop()

	timeoutTimer := time.NewTimer(TimeoutTime)
	log4go.Info("[room(%d)] running...", r.roomID)

LOOP:
	for {
		select {
		case <-r.exitChan:
			log4go.Error("[room(%d)] force exit", r.roomID)
			return
		case c := <-r.inChan:
			id, ok := c.GetExtraData().(uint64)
			if !ok {
				c.Close()
				log4go.Error("[room(%d)] inChan don't have id", r.roomID)
				continue
			}
			if r.g.JoinGame(id, c) {
				log4go.Info("[room(%d)] player[%d] join room ok", r.roomID, id)
			} else {
				log4go.Error("[room(%d)] player[%d] join room failed", r.roomID, id)
				c.Close()
			}
		case c := <-r.outChan:
			id, ok := c.GetExtraData().(uint64)
			if !ok {
				c.Close()
				log4go.Error("[room(%d)] outChan don't have id", r.roomID)
				continue
			}
			r.g.LeaveGame(id)
		case <-tickerTick.C:
			if !r.g.Tick(time.Now().Unix()) {
				log4go.Info("[room(%d)] tick over", r.roomID)
				break LOOP
			}
		case <-timeoutTimer.C:
			log4go.Error("[room(%d)] time out", r.roomID)
			break LOOP
		case msg := <-r.msgQ:
			r.g.ProcessMsg(msg.id, msg.msg.(*pb_packet.Packet))
		}
	}

	r.g.Close()
	for i := 3; i > 0; i-- {
		<-time.After(time.Second)
		log4go.Info("[room(%d)] quiting %d...", r.roomID, i)
	}
}
