package game

import (
	"time"

	"github.com/alecthomas/log4go"
	"github.com/hedon954/go-lock-step-server/pb"
	"github.com/hedon954/go-lock-step-server/pkg/network"
	"github.com/hedon954/go-lock-step-server/pkg/packet/pb_packet"
	"google.golang.org/protobuf/proto"
)

// GameState the state of the game
type GameState int

const (
	k_Ready  GameState = iota // game ready
	k_Gaming                  // gaming
	k_Over                    // game over
	k_Stop                    // game finish
)

const (

	// maximum preparation time.
	// If no one connects after this time, just shut down the game
	MaxReadyTime int64 = 20

	// maximum number of frames per game
	MaxGameFrame uint32 = 30*60*3 + 100

	// the interval of broadcast frame data
	BroadcastOffsetFrames = 3

	// frame data each message packet contains at most
	kMaxFrameDataPerMsg = 60

	// If no heartbeat packet is received during this time period,
	// the network is considered to be poor,
	// and the packet will not be sent continuously
	// (the reading and writing time of the network layer is set relatively long,
	// which is the solution required by the client)
	kBadNetworkThreshold = 2
)

type gameListener interface {
	OnJoinGame(gid uint64, pid uint64)
	OnGameStart(gid uint64)
	OnLeaveGame(gid uint64, pid uint64)
	OnGameOver(gid uint64)
}

// Game represents a game
type Game struct {
	id               uint64
	startTime        int64
	randomSeed       int32
	State            GameState
	players          map[uint64]*Player
	logic            *lockstep
	clientFrameCount uint32
	result           map[uint64]uint64
	listener         gameListener
	dirty            bool
}

// NewGame builds a game meta
func NewGame(id uint64, players []uint64, randomSeed int32, listener gameListener) *Game {
	g := &Game{
		id:         id,
		players:    make(map[uint64]*Player),
		logic:      newLockStep(),
		startTime:  time.Now().Unix(),
		randomSeed: randomSeed,
		listener:   listener,
		result:     make(map[uint64]uint64),
	}

	for i, pid := range players {
		g.players[pid] = NewPlayer(pid, int32(i+1))
	}

	return g
}

// JoinGame user joins game
func (g *Game) JoinGame(pid uint64, conn *network.Conn) bool {

	msg := &pb.S2C_ConnectMsg{
		ErrorCode: pb.ERRORCODE_ERR_ok.Enum(),
	}

	p, ok := g.players[pid]
	if !ok {
		log4go.Error("[game(%d)] player[%d] join room failed", g.id, pid)
		return false
	}

	if g.State != k_Ready && g.State != k_Gaming {
		msg.ErrorCode = pb.ERRORCODE_ERR_RoomState.Enum()
		p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), msg))
		log4go.Error("[game(%d)] player[%d] game is over", g.id, pid)
		return true
	}

	// replace the current player
	if p.client != nil {
		p.client.PutExtraData(nil)
		log4go.Warn("[game(%d)] player[%d] replace", g.id, pid)
	}

	p.Connect(conn)
	p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), msg))
	g.listener.OnJoinGame(g.id, pid)
	return true
}

// LeaveGame user leaves game
func (g *Game) LeaveGame(pid uint64) bool {
	p, ok := g.players[pid]
	if !ok {
		return false
	}
	p.Cleanup()
	g.listener.OnLeaveGame(g.id, pid)
	return true
}

// ProcessMsg handles the message
func (g *Game) ProcessMsg(pid uint64, msg *pb_packet.Packet) {
	player, ok := g.players[pid]
	if !ok {
		log4go.Error("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())
		return
	}
	log4go.Info("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())

	msgID := pb.ID(msg.GetMessageID())
	switch msgID {
	case pb.ID_MSG_JoinRoom:
		jrMsg := &pb.S2C_JoinRoomMsg{
			Roomseatid: proto.Int32(player.idx),
			RandomSeed: proto.Int32(g.randomSeed),
		}
		for _, p := range g.players {
			if p.id == player.id {
				continue
			}
			jrMsg.Others = append(jrMsg.Others, p.id)
			jrMsg.Pros = append(jrMsg.Pros, p.loadingProgress)
		}
		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_JoinRoom), jrMsg))

	case pb.ID_MSG_Progress:
		if g.State > k_Ready {
			break
		}
		m := &pb.S2C_ProgressMsg{}
		if err := msg.UnmarshalPB(m); err != nil {
			log4go.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id,
				msg.GetMessageID(), err.Error())
			return
		}
		player.loadingProgress = m.GetPro()
		pMsg := pb_packet.NewPacket(uint8(pb.ID_MSG_Progress), &pb.S2C_ProgressMsg{
			Id:  proto.Uint64(player.id),
			Pro: m.Pro,
		})
		g.broadcastExclude(pMsg, player.id)

	case pb.ID_MSG_Heartbeat:
		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Heartbeat), nil))
		player.RefreshHeartbeat()

	case pb.ID_MSG_Ready:
		if g.State == k_Ready {
			g.doReady(player)
		} else if g.State == k_Gaming {
			g.doReady(player)
			g.doReconnect(player)
			log4go.Warn("[game(%d)] doReconnect [%d]", g.id, player.id)
		} else {
			log4go.Error("[game(%d)] ID_MSG_Ready player[%d] state error:[%d]", g.id, player.id, g.State)
		}

	case pb.ID_MSG_Input:
		m := &pb.C2S_InputMsg{}
		if err := msg.UnmarshalPB(m); err != nil {
			log4go.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id,
				msg.GetMessageID(), err.Error())
			return
		}
		if !g.pushInput(player, m) {
			log4go.Warn("[game(%d)] processMsg player[%d] msg=[%d] pushInput failed", g.id, player.id,
				msg.GetMessageID())
			break
		}
		// mandatory broadcast of the next frame (required by the client)
		g.dirty = true

	case pb.ID_MSG_Result:
		m := &pb.C2S_ResultMsg{}
		if err := msg.UnmarshalPB(m); err != nil {
			log4go.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id,
				msg.GetMessageID(), err.Error())
			return
		}
		g.result[player.id] = m.GetWinnerID()
		log4go.Info("[game(%d)] ID_MSG_Result player[%d] winner=[%d]", g.id, player.id, m.GetWinnerID())
		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Result), nil))
	default:
		log4go.Warn("[game(%d)] processMsg unknown message id[%d]", msgID)
	}
}

// Tick is the main logic
func (g *Game) Tick(now int64) bool {
	switch g.State {
	case k_Ready:
		delta := now - g.startTime
		if delta < MaxReadyTime {
			if g.checkReady() {
				g.doStart()
				g.State = k_Gaming
			}
		} else {
			if g.getOnlinePlayerCount() > 0 {
				// start once although only one player
				g.doStart()
				g.State = k_Gaming
				log4go.Warn("[game(%d)] force start game because ready state is timeout ", g.id)
			} else {
				// none player, game over
				g.State = k_Over
				log4go.Error("[game(%d)] game over!! nobody ready", g.id)
			}
		}
		return true

	case k_Gaming:
		if g.checkOver() {
			g.State = k_Over
			log4go.Info("[game(%d)] game over successfully!!", g.id)
			return true
		}

		if g.isTimeout() {
			g.State = k_Over
			log4go.Warn("[game(%d)] game timeout", g.id)
			return true
		}

		g.logic.tick()
		g.broadcastFrameData()
		return true

	case k_Over:
		g.doGameOver()
		g.State = k_Stop
		log4go.Info("[game(%d)] do game over", g.id)
		return true
	case k_Stop:
		return false
	}
	return false
}

// broadcast msg
func (g *Game) broadcast(msg network.Packet) {
	for _, v := range g.players {
		v.SendMessage(msg)
	}
}

// broadcastExclude broadcast msg exclude pid
func (g *Game) broadcastExclude(msg network.Packet, pid uint64) {
	for _, p := range g.players {
		if p.id == pid {
			continue
		}
		p.SendMessage(msg)
	}
}

// pushInput pushes input msg to lock step server
func (g *Game) pushInput(p *Player, msg *pb.C2S_InputMsg) bool {
	cmd := &pb.InputData{
		Id:         proto.Uint64(p.id),
		Sid:        proto.Int32(msg.GetSid()),
		X:          proto.Int32(msg.GetX()),
		Y:          proto.Int32(msg.GetY()),
		Roomseatid: proto.Int32(p.idx),
	}

	return g.logic.pushCmd(cmd)
}

// doReady user ready to start game
func (g *Game) doReady(player *Player) {
	if player.isReady {
		return
	}
	player.isReady = true
	msg := pb_packet.NewPacket(uint8(pb.ID_MSG_Ready), nil)
	player.SendMessage(msg)
}

// doReconnect user reconnects to the game
func (g *Game) doReconnect(player *Player) {
	msg := &pb.S2C_StartMsg{
		TimeStamp: proto.Int64(g.startTime),
	}
	ret := pb_packet.NewPacket(uint8(pb.ID_MSG_Start), msg)
	player.SendMessage(ret)

	frameCount := g.clientFrameCount
	c := 0

	frameMsg := &pb.S2C_FrameMsg{}
	var i uint32 = 0
	for i = 0; i < frameCount; i++ {
		fd := g.logic.getFrame(i)
		if fd == nil && i != (frameCount-1) {
			continue
		}
		f := &pb.FrameData{
			FrameID: proto.Uint32(i),
		}
		if fd != nil {
			f.Input = fd.cmds
		}
		frameMsg.Frames = append(frameMsg.Frames, f)
		c++

		if c >= kMaxFrameDataPerMsg || i == (frameCount-1) {
			player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Frame), frameMsg))
			c = 0
			frameMsg = &pb.S2C_FrameMsg{}
		}
	}

	player.SetSendFrameCount(g.clientFrameCount)
}

// checkReady checks if the game is ready
func (g *Game) checkReady() bool {
	for _, p := range g.players {
		if !p.isReady {
			return false
		}
	}
	return true

}

// doStart starts the game
func (g *Game) doStart() {
	g.clientFrameCount = 0
	g.logic.reset()
	for _, p := range g.players {
		p.isReady = true
		p.loadingProgress = 100
	}
	g.startTime = time.Now().Unix()
	msg := &pb.S2C_StartMsg{
		TimeStamp: proto.Int64(g.startTime),
	}
	ret := pb_packet.NewPacket(uint8(pb.ID_MSG_Start), msg)
	g.broadcast(ret)
	g.listener.OnGameStart(g.id)
}

// doGameOver game over
func (g *Game) doGameOver() {
	g.listener.OnGameOver(g.id)
}

// checkOver checks if the game is over
// if there is one player is online or do not send result, game is not over
func (g *Game) checkOver() bool {
	for _, p := range g.players {
		if !p.isOnline {
			continue
		}
		if _, ok := g.result[p.id]; !ok {
			return false
		}
	}
	return true
}

func (g *Game) getPlayer(id uint64) *Player {
	return g.players[id]
}

func (g *Game) getPlayerCount() int {
	return len(g.players)
}

// getOnlinePlayerCount returns the count of online players
func (g *Game) getOnlinePlayerCount() int {
	i := 0
	for _, v := range g.players {
		if v.IsOnline() {
			i++
		}
	}
	return i
}

// isTimeout checks if the game is timeout
func (g *Game) isTimeout() bool {
	return g.logic.getFrameCount() > MaxGameFrame
}

// broadcastFrameData broadcasts frame datas around the game room
func (g *Game) broadcastFrameData() {
	frameCount := g.logic.getFrameCount()
	if !g.dirty && frameCount-g.clientFrameCount < BroadcastOffsetFrames {
		return
	}

	defer func() {
		g.dirty = false
		g.clientFrameCount = frameCount
	}()

	now := time.Now().Unix()

	for _, p := range g.players {
		if !p.isOnline {
			continue
		}
		if !p.isReady {
			continue
		}
		if now-p.GetLastHeartbeatTime() >= kBadNetworkThreshold {
			continue
		}

		i := p.GetSendFrameCount()
		c := 0
		msg := &pb.S2C_FrameMsg{}
		for ; i < frameCount; i++ {
			fd := g.logic.getFrame(i)
			if fd == nil && i != (frameCount-1) {
				continue
			}

			f := &pb.FrameData{
				FrameID: proto.Uint32(i),
			}

			if fd != nil {
				f.Input = fd.cmds
			}
			msg.Frames = append(msg.Frames, f)
			c++

			if i == (frameCount-1) || c >= kMaxFrameDataPerMsg {
				p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Frame), msg))
				c = 0
				msg = &pb.S2C_FrameMsg{}
			}
		}
		p.SetSendFrameCount(frameCount)
	}
}

// Result returns the game result
func (g *Game) Result() map[uint64]uint64 {
	return g.result
}

// Close closes the game
func (g *Game) Close() {
	msg := pb_packet.NewPacket(uint8(pb.ID_MSG_Close), nil)
	g.broadcast(msg)
}

// Cleanup clears the game's info
func (g *Game) Cleanup() {
	for _, p := range g.players {
		p.Cleanup()
	}
	g.players = make(map[uint64]*Player)

}
