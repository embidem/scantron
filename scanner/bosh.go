package scanner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"

	boshcmd "github.com/cloudfoundry/bosh-init/cmd"
	boshconfig "github.com/cloudfoundry/bosh-init/cmd/config"
	boshdir "github.com/cloudfoundry/bosh-init/director"
	boshssh "github.com/cloudfoundry/bosh-init/ssh"
	boshuaa "github.com/cloudfoundry/bosh-init/uaa"
	boshui "github.com/cloudfoundry/bosh-init/ui"
	nmap "github.com/lair-framework/go-nmap"
	"github.com/pivotal-cf/scantron"
	"github.com/pivotal-golang/lager"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type boshScanner struct {
	nmapRun               *nmap.NmapRun
	creds                 boshconfig.Creds
	deploymentName        string
	boshURL               string
	boshUsername          string
	boshPassword          string
	boshLogger            boshlog.Logger
	gatewayUsername       string
	gatewayHost           string
	gatewayPrivateKeyPath string
}

func Bosh(
	nmapRun *nmap.NmapRun,
	deploymentName string,
	boshURL string,
	boshUsername string,
	boshPassword string,
	boshLogger boshlog.Logger,
	uaaClient string,
	uaaClientSecret string,
	gatewayUsername string,
	gatewayHost string,
	gatewayPrivateKeyPath string,
) Scanner {
	return &boshScanner{
		nmapRun: nmapRun,
		creds: boshconfig.Creds{
			Client:       uaaClient,
			ClientSecret: uaaClientSecret,
		},

		deploymentName: deploymentName,

		boshURL:      boshURL,
		boshUsername: boshUsername,
		boshPassword: boshPassword,
		boshLogger:   boshLogger,

		gatewayUsername:       gatewayUsername,
		gatewayHost:           gatewayHost,
		gatewayPrivateKeyPath: gatewayPrivateKeyPath,
	}
}

func (s *boshScanner) Scan(logger lager.Logger) error {
	director, err := getDirector(s.boshURL, s.boshUsername, s.boshPassword, s.creds, s.boshLogger)
	if err != nil {
		logger.Error("failed-to-get-director", err)
		return err
	}

	deployment, err := director.FindDeployment(s.deploymentName)
	if err != nil {
		logger.Error("failed-to-find-deployment", err)
		return err
	}

	vmInfos, err := deployment.VMInfos()
	if err != nil {
		logger.Error("failed-to-get-vm-infos", err)
		return err
	}

	inventory := &scantron.Inventory{}

	for _, vmInfo := range vmInfos {
		inventory.Hosts = append(inventory.Hosts, scantron.Host{
			Name:      fmt.Sprintf("%s/%d", vmInfo.JobName, *vmInfo.Index),
			Addresses: vmInfo.IPs,
		})
	}

	ui := boshui.NewConfUI(s.boshLogger)
	ui.EnableJSON()
	defer ui.Flush()

	deps := boshcmd.NewBasicDeps(ui, s.boshLogger)

	tmpDir, err := ioutil.TempDir("", "scantron")
	if err != nil {
		logger.Error("failed-to-create-temp-dir", err)
		return err
	}

	tmpDirPath, err := deps.FS.ExpandPath(tmpDir)
	if err != nil {
		logger.Error("failed-to-expand-temp-dir-path", err)
		return err
	}
	defer os.RemoveAll(tmpDirPath)

	err = deps.FS.ChangeTempRoot(tmpDirPath)
	if err != nil {
		logger.Error("failed-to-change-temp-root", err)
		return err
	}

	sshSessionFactory := func(o boshssh.ConnectionOpts, r boshdir.SSHResult) boshssh.Session {
		return boshssh.NewSessionImpl(o, boshssh.SessionImplOpts{ForceTTY: true}, r, deps.FS)
	}

	sshWriter := NewMemWriter()

	sshRunner := boshssh.NewNonInteractiveRunner(
		boshssh.NewComboRunner(
			deps.CmdRunner,
			sshSessionFactory,
			signal.Notify,
			sshWriter,
			deps.FS,
			ui,
			s.boshLogger,
		),
	)

	sshOpts, privKey, err := boshdir.NewSSHOpts(deps.UUIDGen)
	if err != nil {
		logger.Error("failed-to-create-ssh-opts", err)
		return err
	}

	// empty/empty means scan all instances of all jobs
	slug := boshdir.NewAllOrPoolOrInstanceSlug("", "")

	sshResult, err := deployment.SetUpSSH(slug, sshOpts)
	if err != nil {
		logger.Error("failed-to-set-up-ssh", err)
		return err
	}
	defer deployment.CleanUpSSH(slug, sshOpts)

	connOpts := boshssh.ConnectionOpts{
		PrivateKey: privKey,

		GatewayUsername:       s.gatewayUsername,
		GatewayHost:           s.gatewayHost,
		GatewayPrivateKeyPath: s.gatewayPrivateKeyPath,
	}

	cmd := "sudo lsof -l -iTCP -sTCP:LISTEN +c0 -Fcn -P -n"
	err = sshRunner.Run(connOpts, sshResult, strings.Split(cmd, " "))
	if err != nil {
		logger.Error("failed-to-run-cmd", err)
		return err
	}

	var scannedServices []ScannedService
	for _, nmapHost := range s.nmapRun.Hosts {
		result := sshWriter.ResultsForHost(nmapHost.Addresses[0].Addr)
		if result != nil {
			processes := ParseLSOFOutput(result.StdoutString())
			for _, nmapPort := range nmapHost.Ports {
				for _, process := range processes {
					if process.HasFileWithPort(nmapPort.PortId) {
						scannedServices = append(scannedServices, ScannedService{
							hostname: result.JobName(),
							ip:       result.Host(),
							name:     process.CommandName,
							port:     nmapPort.PortId,
							ssl:      len(nmapPort.Service.Tunnel) > 0,
						})
					}
				}
			}
		}
	}

	wr := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	defer wr.Flush()

	fmt.Fprintln(wr, strings.Join([]string{"IP Address", "Job", "Service", "Port", "SSL"}, "\t"))

	for _, o := range scannedServices {
		ssl := ""
		if o.ssl {
			ssl = asciiCheckmark
		}

		fmt.Fprintln(wr, fmt.Sprintf("%s\t%s\t%s\t%d\t%s", o.ip, o.hostname, o.name, o.port, ssl))
	}

	return nil
}

func getDirector(
	boshURL string,
	boshUsername string,
	boshPassword string,
	creds boshconfig.Creds,
	logger boshlog.Logger,
) (boshdir.Director, error) {
	dirConfig, err := boshdir.NewConfigFromURL(boshURL)
	if err != nil {
		return nil, err
	}

	uaa, err := getUAA(dirConfig, creds, logger)
	if err != nil {
		return nil, err
	}

	if creds.IsUAAClient() {
		dirConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc
	} else {
		dirConfig.Username = boshUsername
		dirConfig.Password = boshPassword
	}

	director, err := boshdir.NewFactory(logger).New(dirConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}

	return director, nil
}

func getUAA(dirConfig boshdir.Config, creds boshconfig.Creds, logger boshlog.Logger) (boshuaa.UAA, error) {
	director, err := boshdir.NewFactory(logger).New(dirConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}

	info, err := director.Info()
	if err != nil {
		return nil, err
	}

	uaaURL := info.Auth.Options["url"]

	uaaURLStr, ok := uaaURL.(string)
	if !ok {
		return nil, err
	}

	uaaConfig, err := boshuaa.NewConfigFromURL(uaaURLStr)
	if err != nil {
		return nil, err
	}

	uaaConfig.Client = creds.Client
	uaaConfig.ClientSecret = creds.ClientSecret

	return boshuaa.NewFactory(logger).New(uaaConfig)
}
