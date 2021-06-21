package main

import (
	"encoding/json"
	"flag"
	"fmt"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gorilla/websocket"
	"log"
	"mon/src/shared"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var table *widgets.Table

func setupUi() {
	table = widgets.NewTable()
	table.Rows = [][]string{
		{"Hostname", "Online", "Cpu Usage", "Cpu Temp"},
	}
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.RowSeparator = true
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.FillRow = true
	table.RowStyles[0] = ui.NewStyle(ui.ColorWhite, ui.ColorBlack, ui.ModifierBold)
	termWidth, termHeight := ui.TerminalDimensions()
	table.SetRect(0, 0, termWidth, termHeight)
	//table.RowStyles[1] = ui.NewStyle(ui.ColorGreen, ui.ColorBlack)
}

func eventLoop(conn *websocket.Conn) {
	updateTicker := time.NewTicker(time.Second)
	defer updateTicker.Stop()

	sigTerm := make(chan os.Signal, 2)
	signal.Notify(sigTerm, os.Interrupt, syscall.SIGTERM)

	uiEvents := ui.PollEvents()

	for {
		select {
		case <-sigTerm:
			return
		case <-updateTicker.C:
			err := conn.WriteMessage(websocket.TextMessage, []byte("get"))
			if err != nil {
				log.Println("get:", err)
				break
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			//log.Println("message:", string(message))

			var status map[string]shared.Status
			err = json.Unmarshal(message, &status)
			if err != nil {
				log.Println("json:", err)
			}

			//if len(table.Rows) == 1 {
			//	table.Rows = append(table.Rows, []string{status.Hostname, "true", fmt.Sprintf("%f", status.Cpu.Usage), fmt.Sprintf("%f", status.Cpu.Temperature)})
			//}

			//unfound := make([]shared.Status, 0)

			for _, s := range status {
				for i, r := range table.Rows {
					if i == 0 && len(table.Rows) > 1 {
						continue
					}
					if r[0] == s.Hostname {
						table.Rows[i][1] = strconv.FormatBool(s.Online)
						table.Rows[i][2] = fmt.Sprintf("%f", s.Cpu.Usage)
						table.Rows[i][3] = fmt.Sprintf("%f", s.Cpu.Temperature)
						if s.Online {
							table.RowStyles[i] = ui.NewStyle(ui.ColorWhite, ui.ColorBlack)
						} else {
							table.RowStyles[i] = ui.NewStyle(ui.ColorWhite, ui.ColorRed)
						}
						break
					} else if i+1 == len(table.Rows) {
						//unfound = append(unfound, []string{s.Hostname, "true", fmt.Sprintf("%f", s.Cpu.Usage), fmt.Sprintf("%f", s.Cpu.Temperature)})
						table.Rows = append(table.Rows, []string{s.Hostname, strconv.FormatBool(s.Online), fmt.Sprintf("%f", s.Cpu.Usage), fmt.Sprintf("%f", s.Cpu.Temperature)})
					}
				}
			}
			ui.Render(table)
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			}
		}
	}

	//for e := range ui.PollEvents() {
	//	if e.Type == ui.KeyboardEvent {
	//		break
	//	}
	//}
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/send"}
	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
		return
	}
	defer conn.Close()

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	setupUi()

	//ui.Render(table)

	eventLoop(conn)
	//p := widgets.NewParagraph()
	//p.Text = "Hello World!"
	//p.SetRect(0, 0, 25, 5)
	//
	//ui.Render(p)
}
