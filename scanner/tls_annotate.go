package scanner

import (
	"strconv"

	"github.com/pivotal-cf/scantron/scanlog"
	"github.com/pivotal-cf/scantron/tlsscan"
)

type scannerFunc func(logger scanlog.Logger) ([]ScanResult, error)

func (s scannerFunc) Scan(logger scanlog.Logger) ([]ScanResult, error) {
	return s(logger)
}

func AnnotateWithTLSInformation(scanner Scanner) Scanner {
	return scannerFunc(func(logger scanlog.Logger) ([]ScanResult, error) {
		scannedHosts, err := scanner.Scan(logger)
		if err != nil {
			return nil, err
		}

		for j, scannedHost := range scannedHosts {
			services := scannedHost.Services

			for _, hostService := range services {
				ports := hostService.Ports

				for n := range ports {
					port := ports[n]
					if port.State != "LISTEN" {
						continue
					}

					host := scannedHost.IP
					portNum := strconv.Itoa(port.Number)

					results, err := tlsscan.Scan(logger, host, portNum)
					if err != nil {
						port.TLSInformation.ScanError = err
						continue
					}

					if !results.HasTLS() {
						continue
					}

					port.TLSInformation.CipherInformation = results

					cert, mutual, err := FetchTLSInformation(host, portNum)
					if err != nil {
						port.TLSInformation.ScanError = err
					} else {
						port.TLSInformation.Certificate = cert
						port.TLSInformation.Mutual = mutual
					}

					ports[n] = port
				}
			}

			scannedHosts[j].Services = services
		}

		return scannedHosts, nil
	})
}
