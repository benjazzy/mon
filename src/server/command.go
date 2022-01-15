package main

type Type string

const (
	Get      Type = "get"
	Reboot        = "reboot"
	Shutdown      = "shutdown"
)

type Command struct {
	Command Type   `json:"command"`
	Data    string `json:"data"`
}
