package scanner

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pivotal-cf/scantron/remotemachine"
	"github.com/pivotal-cf/scantron/scanlog"
)

type boshScanner struct {
	director remotemachine.BoshDirector
}

func Bosh(director remotemachine.BoshDirector) Scanner {
	return &boshScanner{
		director: director,
	}
}

func (s *boshScanner) Scan(logger scanlog.Logger) ([]ScanResult, error) {
	vms := s.director.VMs()

	wg := &sync.WaitGroup{}
	wg.Add(len(vms))

	hosts := make(chan ScanResult)

	err := s.director.Setup()
	if err != nil {
		return nil, err
	}
	defer s.director.Cleanup()

	for _, vm := range vms {
		vm := vm

		go func() {
			defer wg.Done()

			ip := remotemachine.BestAddress(vm.IPs)

			machineLogger := logger.With(
				"job", vm.JobName,
				"id", vm.ID,
				"index", index(vm.Index),
				"address", fmt.Sprintf("%s", ip),
			)

			remoteMachine := s.director.ConnectTo(machineLogger, vm)
			defer remoteMachine.Close()

			systemInfo, err := scanMachine(machineLogger, remoteMachine)
			if err != nil {
				machineLogger.Errorf("Failed to scan machine: %s", err)
				return
			}

			boshName := fmt.Sprintf("%s/%s", vm.JobName, vm.ID)
			hosts <- buildScanResult(systemInfo, boshName, ip)
		}()
	}

	go func() {
		wg.Wait()
		close(hosts)
	}()

	var scannedHosts []ScanResult

	for host := range hosts {
		scannedHosts = append(scannedHosts, host)
	}

	return scannedHosts, nil
}

func index(index *int) string {
	if index == nil {
		return "?"
	}

	return strconv.Itoa(*index)
}
