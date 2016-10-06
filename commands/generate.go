package commands

import (
	"os"

	"github.com/pivotal-cf/scantron/audit"
)

type GenerateManifestCommand struct {
	Database string `long:"database" description:"path to report database" value-name:"PATH" default:"./database.db"`
}

func (command *GenerateManifestCommand) Execute(args []string) error {
	db, err := OpenDatabase(command.Database)
	if err != nil {
		return err
	}

	return audit.GenerateManifest(os.Stdout, db.DB())
}
