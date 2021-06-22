package main

import (
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	"html/template"
	"log"
	"mon/src/shared"
	"net/http"
	"sync"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

var status = make(map[string]*shared.Status)
var statusLock = &sync.Mutex{}

var timers = make(map[string]*time.Timer)

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/echo")
}

func receive(w http.ResponseWriter, r *http.Request) {
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
			return
		}
		//log.Printf("recv: %s", message)

		var s shared.Status
		err = json.Unmarshal(message, &s)
		if err != nil {
			log.Println("json:", err)
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

		if string(message) == "get" {
			//log.Println("get")
			statusLock.Lock()
			m, err := json.Marshal(status)
			statusLock.Unlock()
			if err != nil {
				log.Println("json:", err)
			}

			err = c.WriteMessage(websocket.TextMessage, m)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/receive", receive)
	http.HandleFunc("/send", send)
	http.HandleFunc("/", home)
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
