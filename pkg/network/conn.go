package network

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alecthomas/log4go"
)

var (
	ErrConnClosing   = errors.New("use of closed network connection")
	ErrWriteBlocking = errors.New("write packet was blocking")
	ErrReadBlocking  = errors.New("read packet was blocking")
)

// Conn exposes a set of callbacks for the various events that occur on a connection
type Conn struct {
	srv               *Server
	conn              net.Conn
	extraData         interface{}
	closeOnce         sync.Once
	closeFlag         int32
	closeChan         chan struct{}
	packetSendChan    chan Packet
	packetReceiveChan chan Packet
	callback          ConnCallback
}

// ConnCallback is an interface of methods that are used as callbacks on a connection
type ConnCallback interface {
	// OnConnect is called when the connection was accepted,
	// If the return value of false is closed
	OnConnect(*Conn) bool

	// OnMessage is called when the connection receives a packet,
	// If the return value of false is closed
	OnMessage(*Conn, Packet) bool

	// OnClose is called when the connection closed
	OnClose(*Conn)
}

// NewConn creates a new connection
func NewConn(conn net.Conn, srv *Server) *Conn {
	return &Conn{
		srv:               srv,
		callback:          srv.callback,
		conn:              conn,
		closeChan:         make(chan struct{}),
		packetSendChan:    make(chan Packet, srv.config.PacketSendChanLimit),
		packetReceiveChan: make(chan Packet, srv.config.PacketReceiveChanLimit),
	}
}

// GetExtraData returns the extra data from the Conn
func (c *Conn) GetExtraData() interface{} {
	return c.extraData
}

// PutExtraData puts the extra data with the Conn
func (c *Conn) PutExtraData(data interface{}) {
	c.extraData = data
}

// SetCallback sets the connection callback
func (c *Conn) SetCallback(callback ConnCallback) {
	c.callback = callback
}

// GetRawConn returns the raw net.TCPConn from the Conn
func (c *Conn) GetRawConn() net.Conn {
	return c.conn
}

// Close closes the connection
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.closeFlag, 1)
		close(c.closeChan)
		close(c.packetSendChan)
		close(c.packetReceiveChan)
		c.conn.Close()
		c.callback.OnClose(c)
	})
}

// IsClosed indicates whether the connection is closed or not
func (c *Conn) IsClosed() bool {
	return atomic.LoadInt32(&c.closeFlag) == 1
}

// AsyncWritePacket async writes a packet, this method woule never be blocked
func (c *Conn) AsyncWritePacket(p Packet, timeout time.Duration) (err error) {
	if c.IsClosed() {
		return ErrConnClosing
	}

	defer func() {
		if e := recover(); e != nil {
			err = ErrConnClosing
		}
	}()

	if timeout == 0 {
		select {
		case c.packetSendChan <- p:
			return nil
		default:
			return ErrWriteBlocking
		}
	} else {
		select {
		case c.packetSendChan <- p:
			return nil
		case <-c.closeChan:
			return ErrConnClosing
		case <-time.After(timeout):
			return ErrWriteBlocking
		}
	}
}

// Do is to run loops
func (c *Conn) Do() {
	if !c.callback.OnConnect(c) {
		return
	}

	asyncDo(c.handleLoop, c.srv.waitGroup)
	asyncDo(c.readLoop, c.srv.waitGroup)
	asyncDo(c.writeLoop, c.srv.waitGroup)
}

func asyncDo(fn func(), wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		fn()
		wg.Done()
	}()
}

// readLoop is a loop to read packet
func (c *Conn) readLoop() {
	defer func() {
		recover()
		c.Close()
	}()

	for {
		select {
		case <-c.srv.exitChan:
			return
		case <-c.closeChan:
			return
		default:
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(c.srv.config.ConnReadTimeout))
		p, err := c.srv.protocol.ReadPacket(c.conn)
		if err != nil {
			return
		}
		c.packetReceiveChan <- p
	}
}

// writeLoop is a loop to write
func (c *Conn) writeLoop() {
	defer func() {
		recover()
		c.Close()
	}()

	for {
		select {
		case <-c.srv.exitChan:
			return
		case <-c.closeChan:
			return
		case p := <-c.packetSendChan:
			if c.IsClosed() {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.srv.config.ConnWriteTimeout))
			if _, err := c.conn.Write(p.Serialize()); err != nil {
				log4go.Error("write packet error: %v\n", err)
				return
			}
		}
	}
}

// handleLoop is a loop to handle OnMessage event
func (c *Conn) handleLoop() {
	defer func() {
		recover()
		c.Close()
	}()
	for {
		select {
		case <-c.srv.exitChan:
			return

		case <-c.closeChan:
			return

		case p := <-c.packetReceiveChan:
			if c.IsClosed() {
				return
			}
			if !c.callback.OnMessage(c, p) {
				return
			}
		}
	}
}
