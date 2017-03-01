package main

import (
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Print("An address argument is required")
		os.Exit(1)
	}
	var addr = os.Args[1]

	chat := &Chat{
		m:           &sync.Mutex{},
		knownUsers:  make([]*User, 0),
		connections: make(map[string]*Connection),
	}

	rpc.Register(chat)

	go func() {
		for {
			chat.closeStaleConnections()
			time.Sleep(time.Second)
		}
	}()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print("accept error:", err)
		}
		log.Printf("New connection: %v", conn.RemoteAddr())
		go rpc.ServeConn(conn)
	}
}
