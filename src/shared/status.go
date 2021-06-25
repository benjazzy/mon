package shared

import (
	"fmt"
	"os"
)

type Status struct {
	Hostname string  `json:"hostname"`
	Online   bool    `json:"online"`
	InConfig bool    `json:"in_config"`
	Cpu      cpuInfo `json:"cpu"`
}

func GetStatus(hostnameConfig []string) Status {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	inConfig := false
	for _, c := range hostnameConfig {
		if c == hostname {
			inConfig = true
			break
		}
	}

	return Status{hostname, true, inConfig, GetCPU()}
}

func NewEmpty(hostname string) Status {
	return Status{hostname, false, true, NewEmptyCPU()}
}
