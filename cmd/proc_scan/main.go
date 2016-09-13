package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	ps "github.com/mitchellh/go-ps"
	"github.com/pivotal-cf/scantron"
	"github.com/pivotal-cf/scantron/scanner"
)

func main() {
	processes, err := ps.Processes()

	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to get process list:", err)
		os.Exit(1)
	}

	jsonProcesses := []scantron.Process{}

	for _, process := range processes {

		pid := process.Pid()

		jsonProcess := scantron.Process{
			CommandName: process.Executable(),
			ID:          pid,
		}

		bs, err := exec.Command("ps", "-e", "-o", "uname:20=", "-f", strconv.Itoa(pid)).CombinedOutput()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting user:", err)
			os.Exit(1)
		}
		jsonProcess.User = strings.TrimSpace(string(bs))

		jsonProcess.Cmdline, err = readFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting cmdline:", err)
			os.Exit(1)
		}

		jsonProcess.Env, err = readFile(fmt.Sprintf("/proc/%d/environ", pid))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting env:", err)
			os.Exit(1)
		}

		getLSOFOutput := func(protocol string) []scantron.Port {
			ports := []scantron.Port{}
			bs, err = exec.Command("lsof",
				fmt.Sprintf("-i4%s", protocol),
				"-a",
				"-p",
				strconv.Itoa(pid),
				"+c0",
				"-FcnL",
				"-n",
				"-P",
			).Output()
			if err == nil {
				lsofProcs := scanner.ParseLSOFOutput(string(bs))

				for _, lsofProc := range lsofProcs {
					for _, file := range lsofProc.Files {
						if address, number, ok := file.Port(); ok {
							ports = append(ports, scantron.Port{Protocol: protocol, Address: address, Number: number})
						}
					}
				}

			}
			return ports
		}

		ports := []scantron.Port{}
		ports = append(ports, getLSOFOutput("TCP")...)
		ports = append(ports, getLSOFOutput("UDP")...)
		jsonProcess.Ports = ports
		jsonProcesses = append(jsonProcesses, jsonProcess)
	}

	json.NewEncoder(os.Stdout).Encode(jsonProcesses)
}

func readFile(path string) ([]string, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return []string{}, err
	}

	inputs := strings.Split(string(bs), "\x00")
	output := []string{}

	for _, input := range inputs {
		if input != "" {
			output = append(output, input)
		}
	}

	return output, nil
}
