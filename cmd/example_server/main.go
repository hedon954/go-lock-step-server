package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/hedon954/go-lock-step-server/cmd/example_server/api"
	"github.com/hedon954/go-lock-step-server/pkg/log4gox"
	"github.com/hedon954/go-lock-step-server/server"
)

var (
	httpAddress = flag.String("web", ":80", "web listen address")
	udpAddress  = flag.String("udp", ":10086", "udp listen address(':10086' means localhost:10086)")
	debugLog    = flag.Bool("log", true, "debug log")
)

func main() {
	flag.Parse()

	log4go.Close()
	log4go.AddFilter("debug logger", log4go.DEBUG, log4gox.NewColorConsoleLogWriter())

	s, err := server.New(*udpAddress)
	if err != nil {
		panic(err)
	}
	_ = api.NewWebAPI(*httpAddress, s.RoomManager())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	ticker := time.NewTimer(time.Minute)
	defer ticker.Stop()

	log4go.Info("[main] start...")
	// 主循环
QUIT:
	for {
		select {
		case sig := <-sigs:
			log4go.Info("Signal: %s", sig.String())
			break QUIT
		case <-ticker.C:
			// todo
			fmt.Println("room number ", s.RoomManager().RoomNum())
		}
	}
	log4go.Info("[main] quiting...")
	s.Stop()
}
