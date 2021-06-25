package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"log"
	"mon/src/shared"
	"net/http"
	"strings"
	"sync"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

var clientSockets = make(map[string]*websocket.Conn)
var socketsLock = &sync.Mutex{}

var status = make(map[string]*shared.Status)
var statusLock = &sync.Mutex{}

func receive(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	_, hostname, err := c.ReadMessage()
	if err != nil {
		log.Println("read:", err)
		return
	}

	socketsLock.Lock()
	clientSockets[string(hostname)] = c
	socketsLock.Unlock()

	timer := time.NewTimer(time.Second * 5)
	timerRunning := false

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		//log.Printf("recv: %s", message)

		var s shared.Status
		err = json.Unmarshal(message, &s)
		if err != nil {
			log.Println("json:", err)
		}

		if viper.IsSet("clients") {
			inConfig := false
			for _, c := range viper.GetStringSlice("clients") {
				if c == s.Hostname {
					inConfig = true
					break
				}
			}
			s.InConfig = inConfig
		}

		statusLock.Lock()
		timer.Reset(time.Second * 5)
		if !timerRunning {
			timerRunning = true
			go func() {
				h := s.Hostname
				//After the timer fires this code unblocks.
				<-timer.C
				statusLock.Lock()
				status[h].Online = false
				statusLock.Unlock()
			}()
		}
		status[s.Hostname] = &s
		statusLock.Unlock()
	}
}

func send(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		//log.Printf("recv: %s", message)

		splitMessage := strings.Fields(string(message))

		if len(splitMessage) == 1 {
			if string(message) == "get" {
				//log.Println("get")
				clients := make(map[string]*shared.Status)
				for _, client := range viper.GetStringSlice("clients") {
					c := shared.NewEmpty(client)
					clients[client] = &c
				}

				statusLock.Lock()
				for hostname, client := range status {
					clients[hostname] = client
				}
				m, err := json.Marshal(clients)
				statusLock.Unlock()
				if err != nil {
					log.Println("json:", err)
				}

				err = c.WriteMessage(websocket.TextMessage, m)
				if err != nil {
					log.Println("write:", err)
					break
				}
			} else {
				log.Println("unknown command:", string(message))
			}
		} else if len(splitMessage) == 2 {
			if splitMessage[0] == "reboot" {
				log.Println("Restarting:", splitMessage[1])

				statusLock.Lock()
				client, ok := status[splitMessage[1]]
				statusLock.Unlock()
				if ok {
					if client.Online {
						socketsLock.Lock()
						if conn, ok := clientSockets[splitMessage[1]]; ok {
							err := conn.WriteMessage(websocket.TextMessage, []byte("reboot"))
							if err != nil {
								log.Println("Error sending restart command to:", splitMessage[1])
							}
						}
						socketsLock.Unlock()
					}
				}
			} else if splitMessage[0] == "add" {
				statusLock.Lock()
				client, ok := status[splitMessage[1]]
				statusLock.Unlock()
				if ok {
					contains := false
					for _, c := range viper.GetStringSlice("clients") {
						if c == client.Hostname {
							contains = true
							break
						}
					}
					if !contains {
						//log.Println("add")
						viper.Set("clients", append(viper.GetStringSlice("clients"), client.Hostname))
						err := viper.WriteConfig()
						if err != nil {
							log.Println("config:", err)
						}
					} else {
						log.Println("client ", client.Hostname, "already in config")
					}
				}
			} else if splitMessage[0] == "remove" {
				if viper.IsSet("clients") {
					for i, c := range viper.GetStringSlice("clients") {
						if c == splitMessage[1] {
							viper.Set("clients", remove(viper.GetStringSlice("clients"), i))
							err := viper.WriteConfig()
							if err != nil {
								log.Println("config:", err)
							}
							break
						}
					}
				}
			} else {
				log.Println("unknown command:", string(message))
			}
		} else {
			log.Println("Error parsing command:", string(message))
		}
	}
}

func remove(s []string, i int) []string {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func main() {
	viper.SetConfigName("mons")  // name of config file (without extension)
	viper.SetConfigType("json")  // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/") // path to look for the config file in
	viper.AddConfigPath(".")     // optionally look for config in the working directory
	err := viper.ReadInConfig()  // Find and read the config file
	if err != nil {              // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}

	flag.Parse()
	log.SetFlags(0)
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	keepAlive := time.NewTicker(time.Minute)

	go func() {
		for {
			<-keepAlive.C
			socketsLock.Lock()
			for client, conn := range clientSockets {
				err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
				if err != nil {
					log.Println("ping:", err)
					delete(clientSockets, client)
				}
			}
			socketsLock.Unlock()
		}
	}()

	http.HandleFunc("/receive", receive)
	http.HandleFunc("/send", send)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
