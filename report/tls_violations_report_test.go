package report_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/scantron/db"
	"github.com/pivotal-cf/scantron/report"
)

var _ = Describe("BuildTLSViolationsReport", func() {
	var (
		databasePath string
		database     *db.Database
	)

	BeforeEach(func() {
		databaseFile, err := ioutil.TempFile("", "database.db")
		Expect(err).NotTo(HaveOccurred())
		databaseFile.Close()

		databasePath = databaseFile.Name()

		database, err = createTestDatabase(databasePath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := database.Close()
		Expect(err).NotTo(HaveOccurred())

		err = os.Remove(databasePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("shows processes using non-approved protocols or cipher suites", func() {
		r, err := report.BuildTLSViolationsReport(database)
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Title).To(Equal("Processes using non-approved SSL/TLS settings:"))

		Expect(r.Header).To(Equal([]string{
			"Identity",
			"Port",
			"Process Name",
			"Non-approved Protocol(s)",
			"Non-approved Cipher(s)",
		}))

		Expect(r.Rows).To(HaveLen(3))
		Expect(r.Rows).To(Equal([][]string{
			{"host1", " 7890", "command1", "VersionSSL30", ""},
			{"host1", " 8890", "command1", "", "Bad Cipher"},
			{"host3", " 7890", "command1", "VersionSSL30", "Just the worst"},
		}))
	})
})
