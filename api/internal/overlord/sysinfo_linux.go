package overlord

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// cpuPercent samples /proc/stat over a short interval and returns overall CPU%.
func cpuPercent() float64 {
	idle1, total1 := readCPUStat()
	time.Sleep(200 * time.Millisecond)
	idle2, total2 := readCPUStat()

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta <= 0 {
		return 0
	}
	return (1.0 - idleDelta/totalDelta) * 100.0
}

func readCPUStat() (idle, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0
			}
			for i := 1; i < len(fields); i++ {
				v, _ := strconv.ParseUint(fields[i], 10, 64)
				total += v
				if i == 4 { // idle is 4th value (user, nice, system, idle, ...)
					idle = v
				}
			}
			return idle, total
		}
	}
	return 0, 0
}

// memPercent reads /proc/meminfo and returns used memory percentage.
func memPercent() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var totalKB, availKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMemInfoValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availKB = parseMemInfoValue(line)
		}
	}
	if totalKB == 0 {
		return 0
	}
	return float64(totalKB-availKB) / float64(totalKB) * 100.0
}

func parseMemInfoValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		return v
	}
	return 0
}
