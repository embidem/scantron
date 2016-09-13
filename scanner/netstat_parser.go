package scanner

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/pivotal-cf/scantron"
)

type NetstatInfo struct {
	CommandName    string
	ID             string
	LocalAddress   string
	ForeignAddress string
	State          string
	Protocol       string
}

type NetstatPort struct {
	CommandName string
	ID          int
	Local       scantron.Port
	Foreign     scantron.Port
	State       string
}

func ParseNetstatLine(line string) NetstatInfo {

	netstat := strings.Fields(line)

	if len(netstat) < 7 {
		return NetstatInfo{}
	}
	protocol := netstat[0]
	if !((protocol == "tcp") || (protocol == "udp")) {
		return NetstatInfo{}
	}

	out := strings.Split(netstat[6], "/")
	if len(out) < 2 {
		return NetstatInfo{}
	}

	id := out[0]
	cmd := out[1]

	return NetstatInfo{
		CommandName:    cmd,
		ID:             id,
		LocalAddress:   netstat[3],
		ForeignAddress: netstat[4],
		State:          netstat[5],
		Protocol:       protocol,
	}
}

func CreatePortFromAddress(infoAddress string, protocol string, state string) scantron.Port {

	localPortInfo := strings.Split(infoAddress, ":")
	address := localPortInfo[0]
	number, _ := strconv.Atoi(localPortInfo[1])

	return scantron.Port{
		Protocol: protocol,
		Address:  address,
		Number:   number,
		State:    state,
	}
}

func CreateNetstatPort(info NetstatInfo) NetstatPort {
	localPort := CreatePortFromAddress(info.LocalAddress, info.Protocol, info.State)
	foreignPort := CreatePortFromAddress(info.ForeignAddress, info.Protocol, info.State)

	id, _ := strconv.Atoi(info.ID)
	return NetstatPort{
		CommandName: info.CommandName,
		ID:          id,
		State:       info.State,
		Local:       localPort,
		Foreign:     foreignPort,
	}

}

func ParseNetstatOutput(output string) []NetstatInfo {
	scanner := bufio.NewScanner(strings.NewReader(output))
	result := []NetstatInfo{}

	for scanner.Scan() {
		line := scanner.Text()

		info := ParseNetstatLine(line)

		if len(info.CommandName) == 0 {
			continue
		}

		result = append(result, info)
	}

	return result
}

func ParseNetstatOutputForPort(output string) []NetstatPort {
	scanner := bufio.NewScanner(strings.NewReader(output))
	result := []NetstatPort{}

	for scanner.Scan() {
		line := scanner.Text()

		info := ParseNetstatLine(line)

		if len(info.CommandName) == 0 {
			continue
		}

		port := CreateNetstatPort(info)
		result = append(result, port)
	}

	return result
}
