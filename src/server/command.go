package main

type Type string

const (
	Get Type		= "get"
	Reboot Type		= "reboot"
	Shutdown Type   = "shutdown"
	Add	Type		= "add"
	Remove Type 	= "remove"
)

type Command struct {
	Command Type   `json:"command"`
	Data    string `json:"data"`
}
