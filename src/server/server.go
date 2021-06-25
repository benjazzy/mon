package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"html/template"
	"log"
	"mon/src/shared"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

var clientSockets = make(map[string]*websocket.Conn)

var status = make(map[string]*shared.Status)
var statusLock = &sync.Mutex{}

var timers = make(map[string]*time.Timer)

//func echo(w http.ResponseWriter, r *http.Request) {
//	c, err := upgrader.Upgrade(w, r, nil)
//	if err != nil {
//		log.Print("upgrade:", err)
//		return
//	}
//	defer c.Close()
//	for {
//		mt, message, err := c.ReadMessage()
//		if err != nil {
//			log.Println("read:", err)
//			break
//		}
//		log.Printf("recv: %s", message)
//		err = c.WriteMessage(mt, message)
//		if err != nil {
//			log.Println("write:", err)
//			break
//		}
//	}
//}
//
//func home(w http.ResponseWriter, r *http.Request) {
//	homeTemplate.Execute(w, "ws://"+r.Host+"/echo")
//}

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

	clientSockets[string(hostname)] = c

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
		if _, ok := timers[s.Hostname]; ok {
			if !status[s.Hostname].Online {
				timers[s.Hostname] = time.NewTimer(time.Second * 5)
				go func() {
					h := s.Hostname
					//After the timer fires this code unblocks.
					<-timers[h].C
					statusLock.Lock()
					status[h].Online = false
					statusLock.Unlock()
				}()
			}
		}
		status[s.Hostname] = &s
		statusLock.Unlock()

		if _, ok := timers[s.Hostname]; ok {
			timers[s.Hostname].Reset(time.Second * 5)
		} else {
			timers[s.Hostname] = time.NewTimer(time.Second * 5)
			go func() {
				h := s.Hostname
				//After the timer fires this code unblocks.
				<-timers[h].C
				statusLock.Lock()
				status[h].Online = false
				statusLock.Unlock()
			}()
		}

		//err = c.WriteMessage(mt, message)
		//if err != nil {
		//	log.Println("write:", err)
		//}
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

				for hostname, client := range status {
					clients[hostname] = client
				}

				statusLock.Lock()
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
				if client, ok := status[splitMessage[1]]; ok {
					if client.Online {
						if conn, ok := clientSockets[splitMessage[1]]; ok {
							err := conn.WriteMessage(websocket.TextMessage, []byte("reboot"))
							if err != nil {
								log.Println("Error sending restart command to:", splitMessage[1])
							}
						}
					}
				}
			} else if splitMessage[0] == "add" {
				if client, ok := status[splitMessage[1]]; ok {
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
			} else if splitMessage[0] == "check" {
				if client, ok := status[splitMessage[1]]; ok {
					contains := false
					for _, c := range viper.GetStringSlice("clients") {
						if c == client.Hostname {
							contains = true
							break
						}
					}
					err := c.WriteMessage(websocket.TextMessage, []byte(client.Hostname+" "+strconv.FormatBool(contains)))
					if err != nil {
						log.Println("write:", err)
						break
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
	//http.HandleFunc("/echo", echo)
	http.HandleFunc("/receive", receive)
	http.HandleFunc("/send", send)
	//http.HandleFunc("/", home)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output" style="max-height: 70vh;overflow-y: scroll;"></div>
</td></tr></table>
</body>
</html>
`))
