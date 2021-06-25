package shared

import (
	"fmt"
	. "github.com/klauspost/cpuid/v2"
	"github.com/mackerelio/go-osstat/cpu"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type cpuInfo struct {
	Type        string     `json:"type"`
	Usage       float64    `json:"usage"`
	Load        [3]float64 `json:"load"`
	Temperature float64    `json:"temperature"`
}

func GetCPU() cpuInfo {
	before, err := cpu.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	time.Sleep(time.Duration(1) * time.Second)
	after, err := cpu.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	total := float64(after.Total - before.Total)

	t := CPU.BrandName
	u := 100 - (float64(after.Idle-before.Idle) / total * 100)
	var l [3]float64
	temp, err := getTemp()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	return cpuInfo{t, u, l, temp}
}

func getTemp() (temp float64, err error) {
	contents, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(contents), "\n")

	temp, err = strconv.ParseFloat(lines[0], 64)
	if err != nil {
		return 0, err
	}

	temp = temp / 1000

	return
}

func NewEmptyCPU() cpuInfo {
	return cpuInfo{
		Type:        "",
		Usage:       0,
		Load:        [3]float64{},
		Temperature: 0,
	}
}

//func getCPUSample() (idle, total uint64) {
//	contents, err := ioutil.ReadFile("/proc/stat")
//	if err != nil {
//		return
//	}
//	lines := strings.Split(string(contents), "\n")
//	for _, line := range lines {
//		fields := strings.Fields(line)
//		if fields[0] == "cpu" {
//			numFields := len(fields)
//			for i := 1; i < numFields; i++ {
//				val, err := strconv.ParseUint(fields[i], 10, 64)
//				if err != nil {
//					fmt.Println("Error: ", i, fields[i], err)
//				}
//				total += val // tally up all the numbers to get total ticks
//				if i == 4 {  // idle is the 5th field in the cpu line
//					idle = val
//				}
//			}
//			return
//		}
//	}
//	return
//}
