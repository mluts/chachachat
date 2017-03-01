package main

import (
	"bufio"
	"github.com/mluts/chachachat/common"
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"strings"
)

var shouldExit = false

func poll(client *rpc.Client, handle *common.Handle) {
	var (
		msg string
		err error
	)
	for {
		err = client.Call("Chat.Poll", handle, &msg)
		if shouldExit {
			return
		}

		if err != nil && err.Error() == common.ErrTimeout.Error() {
			continue
		}

		if err != nil {
			log.Print("poll error:", err)
			os.Exit(1)
		}

		log.Print(strings.TrimSpace(msg))
	}
}

func waitInterrupt() {
	var sig = make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)
	<-sig
}

func main() {
	if len(os.Args) < 3 {
		log.Print("Expected 2 arguments: server address and username")
		os.Exit(1)
	}

	var addr, username = os.Args[1], os.Args[2]
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		log.Fatal("rpc dial error:", err)
	}

	handle := new(common.Handle)
	err = client.Call("Chat.Login", username, &handle)
	if err != nil {
		log.Fatal("login error:", err)
	}

	go poll(client, handle)
	go func() {
		waitInterrupt()
		shouldExit = true
		log.Print("Exiting...")
		client.Call("Chat.Logout", handle, &common.Nothing{})
		os.Exit(0)
	}()

	for {
		stdin := bufio.NewReader(os.Stdin)
		for {
			msg, err := stdin.ReadString('\n')
			if err != nil {
				log.Print("stdin error:", err)
				continue
			}
			err = client.Call("Chat.Write", common.Message{
				*handle,
				msg,
			}, &common.Nothing{})

			if err != nil {
				log.Print("Chat.Write error:", err)
			}
		}
	}
}
