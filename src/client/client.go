package main

import (
	"encoding/json"
	"flag"
	"golang.org/x/sys/unix"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"

	"mon/src/shared"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func connect(u url.URL) (*websocket.Conn, error) {
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err == nil {
		go func() {
			receive(c)
		}()
	}

	return c, err
}

func receive(c *websocket.Conn) {
	for {
		_, message, err := c.ReadMessage()
		log.Println(string(message))
		if err != nil {
			log.Println("read:", err)
			break
		}
		switch shared.ClientCommand(string(message)) {
		case shared.Ping:
			log.Printf("recv: %s", message)
		case shared.Reboot:
			unixReboot(unix.LINUX_REBOOT_CMD_RESTART)
		case shared.Shutdown:
			unixReboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
		}
		//if string(message) != "ping" {
		//	log.Printf("recv: %s", message)
		//}
		//if string(message) == "reboot" {
		//	unix.Sync()
		//	err := unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
		//	if err != nil {
		//		log.Println("Failed to reboot:", err)
		//	}
		//	os.Exit(0)
		//}
	}
}

func unixReboot(c int) {
	log.Println("Rebooting")
	unix.Sync()
	err := unix.Reboot(c)
	if err != nil {
		log.Println("Failed to reboot:", err)
	}
	os.Exit(0)
}

func sendStatus(c *websocket.Conn) error {
	hostnameConfig := make([]string, 0)
	j, err := json.Marshal(shared.GetStatus(hostnameConfig))
	if err != nil {
		log.Println("json:", err)
	}
	return c.WriteMessage(websocket.TextMessage, j)
}

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
		//c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		c, err = connect(u)
		if err != nil {
			log.Println("Failed")
			time.Sleep(time.Second * 10)
		} else {
			log.Println("Connected")
			break
		}
	}
	defer c.Close()

	//hostname, err := os.Hostname()
	//if err != nil {
	//	log.Println("Error getting hostname")
	//	return
	//}

	err = sendStatus(c)
	if err != nil {
		log.Println("write:", err)
	}

	done := make(chan struct{})

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
			//c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
			c, err = connect(u)
			if err != nil {
				log.Println("Failed")
			} else {
				log.Println("Connected")
				ticker.Reset(time.Second * 1)
				reconnect.Stop()
			}
		case <-ticker.C:
			err = sendStatus(c)
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
