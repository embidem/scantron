package ssh_test

import (
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	scantronssh "github.com/pivotal-cf/scantron/ssh"

	"github.com/onsi/gomega/gstruct"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

var _ = Describe("SshScanner", func() {
	Context("with an SSH server", func() {
		var listener net.Listener

		BeforeEach(func() {
			var err error

			listener, err = net.Listen("tcp", "127.0.0.1:0") //:0 is random port
			Expect(err).NotTo(HaveOccurred())

			startSshServer(listener)
		})

		AfterEach(func() {
			listener.Close()
		})

		It("prints the key type for rsa keys", func() {
			address := listener.Addr().String()

			sshKeys, err := scantronssh.ScanSSH(address)
			Expect(err).NotTo(HaveOccurred())

			expectedKeyTypes := []string{
				"ssh-rsa",
				"ssh-dss",
				"ecdsa-sha2-nistp256",
				"ecdsa-sha2-nistp384",
				"ecdsa-sha2-nistp521",
				"ssh-ed25519",
			}

			for _, key := range expectedKeyTypes {
				Expect(sshKeys).To(ContainElement(gstruct.MatchAllFields(gstruct.Fields{
					"Type": Equal(key),
					"Key":  HavePrefix("AAAA"),
				})))
			}
		})
	})
})

func startSshServer(listener net.Listener) {
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			return nil, errors.New("You shall not pass")
		},
	}

	addKey(config, generateRsaKey())
	addKey(config, generateDsaKey())
	addKey(config, generateEcdsaKey(elliptic.P256()))
	addKey(config, generateEcdsaKey(elliptic.P384()))
	addKey(config, generateEcdsaKey(elliptic.P521()))
	addKey(config, generateEd25519Key())

	go func() {
		defer GinkgoRecover()

		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			ssh.NewServerConn(conn, config)
		}
	}()
}

func addKey(config *ssh.ServerConfig, key interface{}) {
	signer, err := ssh.NewSignerFromKey(key)
	Expect(err).NotTo(HaveOccurred())

	config.AddHostKey(signer)
}

func generateRsaKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	Expect(err).NotTo(HaveOccurred())

	return key
}

func generateDsaKey() *dsa.PrivateKey {
	key := &dsa.PrivateKey{}

	dsa.GenerateParameters(&key.Parameters, rand.Reader, dsa.L1024N160)

	err := dsa.GenerateKey(key, rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	return key
}

func generateEcdsaKey(curve elliptic.Curve) *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	return key
}

func generateEd25519Key() ed25519.PrivateKey {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	return key
}
