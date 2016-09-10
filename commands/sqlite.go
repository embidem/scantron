package commands

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pivotal-cf/scantron/scanner"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(createDDL)
	if err != nil {
		return nil, err
	}

	return &Database{
		db: db,
	}, nil
}

func (db *Database) Close() error {
	return db.db.Close()
}

func (db *Database) SaveReport(scans []scanner.ScannedService) error {
	for _, scan := range scans {
		var hostID int
		queryDDL := "SELECT id FROM hosts WHERE name = ? AND ip = ?"
		err := db.db.QueryRow(queryDDL, scan.Job, scan.IP).Scan(&hostID)
		switch {
		case err == sql.ErrNoRows:
			insertDDL := "INSERT INTO hosts(name, ip) VALUES (?, ?)"
			res, err := db.db.Exec(insertDDL, scan.Job, scan.IP)
			if err != nil {
				return err
			}

			insertedID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			hostID = int(insertedID)
		case err != nil:
			return err
		}

		insertDDL := "INSERT INTO processes(host_id, name, pid, cmdline, user) VALUES (?, ?, ?, ?, ?)"
		cmdline := strings.Join(scan.Cmd.Cmdline, " ")
		res, err := db.db.Exec(insertDDL, hostID, scan.Job, scan.PID, cmdline, scan.User)
		if err != nil {
			return err
		}

		processID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		insertDDL = "INSERT INTO ports(process_id, number) VALUES (?, ?)"
		res, err = db.db.Exec(insertDDL, processID, scan.Port)
		if err != nil {
			return err
		}

		tls := scan.TLSInformation
		if tls.Certificate != nil {
			portID, err := res.LastInsertId()
			if err != nil {
				return err
			}

			tlsCertSubject := tls.Certificate.Subject

			insertDDL = "INSERT INTO tls_informations(port_id, cert_expiration, cert_bits, cert_country, cert_province, cert_locality, cert_organization, cert_common_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
			_, err = db.db.Exec(insertDDL, portID, tls.Certificate.Expiration, tls.Certificate.Bits, tlsCertSubject.Country, tlsCertSubject.Province, tlsCertSubject.Locality, tlsCertSubject.Organization, tlsCertSubject.CommonName)
			if err != nil {
				return err
			}
		}

		insertDDL = "INSERT INTO env_vars(var, process_id) VALUES (?, ?)"
		_, err = db.db.Exec(insertDDL, strings.Join(scan.Cmd.Env, " "), processID)
		if err != nil {
			return err
		}
	}

	return nil
}

var createDDL = `
CREATE TABLE hosts (
	id integer PRIMARY KEY AUTOINCREMENT,
	name text,
	ip text,
	UNIQUE(ip, name)
);

CREATE TABLE processes (
	id integer PRIMARY KEY AUTOINCREMENT,
	host_id integer,
	name text,
	pid integer,
	cmdline text,
	user text,
	FOREIGN KEY(host_id) REFERENCES hosts(id)
);

CREATE TABLE ports (
	id integer PRIMARY KEY AUTOINCREMENT,
	process_id integer,
	number integer,
	FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE TABLE tls_informations(
	id integer PRIMARY KEY AUTOINCREMENT,
	port_id integer,
	cert_expiration datetime,
	cert_bits integer,
	cert_country string,
	cert_province string,
	cert_locality string,
	cert_organization string,
	cert_common_name string,
	FOREIGN KEY(port_id) REFERENCES ports(id)
);

CREATE TABLE env_vars (
	id integer PRIMARY KEY AUTOINCREMENT,
	process_id integer,
	var text,
	FOREIGN KEY(process_id) REFERENCES processes(id)
);
`
