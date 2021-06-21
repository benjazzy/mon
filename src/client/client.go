package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"

	"mon/src/shared"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/receive"}
	log.Printf("connecting to %s", u.String())

	var c *websocket.Conn
	var err error

	for {
		log.Println("Connecting")
		c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		if err == nil {
			log.Println("Connected")
			break
		}
		log.Println("Failed")
		time.Sleep(time.Second * 10)
	}
	defer c.Close()

	done := make(chan struct{})

	//go func() {
	//	defer close(done)
	//	for {
	//		_, message, err := c.ReadMessage()
	//		if err != nil {
	//			log.Println("read:", err)
	//			return
	//		}
	//		log.Printf("recv: %s", message)
	//	}
	//}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	reconnect := time.NewTicker(time.Second * 10)
	reconnect.Stop()

	for {
		select {
		case <-done:
			return
		case <-reconnect.C:
			log.Println("Reconnecting")
			c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
			if err == nil {
				log.Println("Connected")
				ticker.Reset(time.Second * 1)
				reconnect.Stop()
			} else {
				log.Println("Failed")
			}
		case <-ticker.C:
			j, err := json.Marshal(shared.GetStatus())
			if err != nil {
				log.Println("json:", err)
			}
			err = c.WriteMessage(websocket.TextMessage, j)
			if err != nil {
				log.Println("write:", err)
				ticker.Stop()
				reconnect.Reset(time.Second * 10)
				break
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			log.Println("closing")
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
