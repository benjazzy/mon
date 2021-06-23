package shared

import (
	"fmt"
	"os"
)

type Status struct {
	Hostname string  `json:"hostname"`
	Online   bool    `json:"online"`
	Cpu      cpuInfo `json:"cpu"`
}

func GetStatus() Status {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	return Status{hostname, true, GetCPU()}
}
