package network

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/xtaci/kcp-go"
)

const (
	latency = time.Millisecond * 1000
)

// Test_KCPServer tests kcp server
func Test_KCPServer(t *testing.T) {
	l, err := kcp.Listen(":10086")
	if err != nil {
		t.Error(err)
		return
	}

	config := &Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
		ConnReadTimeout:        latency,
		ConnWriteTimeout:       latency,
	}

	callback := &testCallback{}
	server := NewServer(config, callback, &DefaultProtocol{})

	go server.Start(l, func(conn net.Conn, i *Server) *Conn {
		kcpConn := conn.(*kcp.UDPSession)
		kcpConn.SetNoDelay(1, 10, 2, 1)
		kcpConn.SetStreamMode(true)
		kcpConn.SetWindowSize(4096, 4096)
		kcpConn.SetReadBuffer(4 * 1024 * 1024)
		kcpConn.SetWriteBuffer(4 * 1024 * 1024)
		kcpConn.SetACKNoDelay(true)

		return NewConn(conn, i)
	})

	defer server.Stop()
	time.Sleep(1 * time.Second)

	// wg is used to control this test program
	wg := sync.WaitGroup{}

	// start clients to send packet to kcp server
	const maxConn = 100
	for i := 0; i < maxConn; i++ {
		wg.Add(1)
		time.Sleep(1 * time.Nanosecond)
		go func() {
			defer wg.Done()

			c, err := kcp.Dial("127.0.0.1:10086")
			if err != nil {
				t.Errorf("connect kcp server failed: %v", err)
				return
			}
			defer c.Close()

			_, err = c.Write(NewDefaultPacket([]byte("ping")).Serialize())
			if err != nil {
				t.Errorf("ping kcp server failed: %v", err)
				return
			}

			b := make([]byte, 1024)
			_ = c.SetReadDeadline(time.Now().Add(time.Second))
			if _, err = c.Read(b); err != nil {
				t.Errorf("read from kcp server failed: %v", err)
				return
			}
		}()
	}

	// wait for kcp server to pong clients
	wg.Wait()
	time.Sleep(2 * time.Second)

	n := atomic.LoadUint32(&callback.numConn)
	if n != maxConn {
		t.Errorf("numConn[%d] should be [%d]", n, maxConn)
	}

	n = atomic.LoadUint32(&callback.numMsg)
	if n != maxConn {
		t.Errorf("numMsg[%d] should be [%d]", n, maxConn)
	}

	n = atomic.LoadUint32(&callback.numDiscon)
	if n != maxConn {
		t.Errorf("numDiscon[%d] should be [%d]", n, maxConn)
	}
}

func Benchmark_KCPServer(b *testing.B) {
	l, err := kcp.Listen(":10086")
	if err != nil {
		b.Error(err)
		return
	}

	config := &Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
		ConnReadTimeout:        latency,
		ConnWriteTimeout:       latency,
	}

	callback := &testCallback{}

	server := NewServer(config, callback, &DefaultProtocol{})
	go server.Start(l, func(conn net.Conn, i *Server) *Conn {
		kcpConn := conn.(*kcp.UDPSession)
		kcpConn.SetNoDelay(1, 10, 2, 1)
		kcpConn.SetStreamMode(true)
		kcpConn.SetWindowSize(4096, 4096)
		kcpConn.SetReadBuffer(4 * 1024 * 1024)
		kcpConn.SetWriteBuffer(4 * 1024 * 1024)
		kcpConn.SetACKNoDelay(true)

		return NewConn(conn, server)
	})
	defer server.Stop()
	time.Sleep(time.Millisecond * 100)

	wg := sync.WaitGroup{}
	var connCount uint32 = 0
	c, err := kcp.Dial("127.0.0.1:10086")
	if err != nil {
		b.Errorf("dial kcp server failed: %v", err)
		return
	}

	// starts a goroutine to read from kcp server
	go func() {
		for {
			buf := make([]byte, 1024)
			if _, err = c.Read(buf); err != nil {
				return
			}
			wg.Done()
		}
	}()

	// write to kcp server
	for i := 0; i < b.N; i++ {
		connCount++
		wg.Add(1)
		go func() {
			_, err = c.Write(NewDefaultPacket([]byte("ping")).Serialize())
			if err != nil {
				b.Errorf("write to kcp server failed: %v", err)
			}
		}()
	}

	wg.Wait()

	n := atomic.LoadUint32(&callback.numMsg)
	b.Logf("numMsg[%d]", n)
	if callback.numMsg != connCount {
		b.Errorf("numMsg[%d] should be [%d]", n, connCount)
	}
}

type testCallback struct {
	numConn   uint32
	numMsg    uint32
	numDiscon uint32
}

func (t *testCallback) OnConnect(conn *Conn) bool {
	id := atomic.AddUint32(&t.numConn, 1)
	conn.PutExtraData(id)
	return true
}

func (t *testCallback) OnMessage(conn *Conn, packet Packet) bool {
	atomic.AddUint32(&t.numMsg, 1)
	err := conn.AsyncWritePacket(NewDefaultPacket([]byte("pong")), time.Second*1)
	if err != nil {
		log4go.Error("[OnMessage] AsyncWritePacket failed: %v", err)
		return false
	}
	return true
}

func (t *testCallback) OnClose(conn *Conn) {
	atomic.AddUint32(&t.numDiscon, 1)
}
