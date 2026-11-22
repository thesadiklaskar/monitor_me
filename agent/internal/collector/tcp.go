package collector

import (
	"errors"
	"strconv"
	"strings"
)

var tcpStateNames = map[string]string{
	"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV",
	"04": "FIN_WAIT1", "05": "FIN_WAIT2", "06": "TIME_WAIT",
	"07": "CLOSE", "08": "CLOSE_WAIT", "09": "LAST_ACK",
	"0A": "LISTEN", "0B": "CLOSING", "0C": "NEW_SYN_RECV",
}

func parseTCPTable(raw, family string) (map[string]int, map[uint16]int, error) {
	if family != "ipv4" && family != "ipv6" {
		return nil, nil, errors.New("TCP address family must be ipv4 or ipv6")
	}
	states := make(map[string]int)
	listeners := make(map[uint16]int)
	for lineNumber, line := range strings.Split(raw, "\n") {
		if lineNumber == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		separator := strings.LastIndex(fields[1], ":")
		if separator < 0 {
			continue
		}
		portValue, err := strconv.ParseUint(fields[1][separator+1:], 16, 16)
		if err != nil {
			continue
		}
		stateCode := strings.ToUpper(fields[3])
		stateName, known := tcpStateNames[stateCode]
		if !known {
			stateName = "UNKNOWN"
		}
		states[stateName]++
		if stateCode == "0A" {
			listeners[uint16(portValue)]++
		}
	}
	return states, listeners, nil
}
