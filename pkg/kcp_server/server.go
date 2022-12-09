package kcp_server

import (
	"net"
	"time"

	"github.com/hedon954/go-lock-step-server/pkg/network"
	"github.com/xtaci/kcp-go"
)

func ListenAndServe(addr string, callback network.ConnCallback, protocol network.Protocol) (*network.Server, error) {
	dupConfig := &network.Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
		ConnReadTimeout:        time.Second * 5,
		ConnWriteTimeout:       time.Second * 5,
	}

	l, err := kcp.Listen(addr)
	if err != nil {
		return nil, err
	}

	server := network.NewServer(dupConfig, callback, protocol)
	go server.Start(l, func(conn net.Conn, i *network.Server) *network.Conn {

		kcpConn := conn.(*kcp.UDPSession)
		kcpConn.SetNoDelay(1, 10, 2, 1)
		kcpConn.SetStreamMode(true)
		kcpConn.SetWindowSize(4096, 4096)
		kcpConn.SetReadBuffer(4 * 1024 * 1024)
		kcpConn.SetWriteBuffer(4 * 1024 * 1024)
		kcpConn.SetACKNoDelay(true)

		return network.NewConn(conn, i)
	})

	return server, nil
}
