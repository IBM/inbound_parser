package db

import (
	"database/sql"
	"errors"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"

	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func GetDb() *sql.DB {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./inbound_parser_db.sqlite"
	}
	migrationNeeded := false
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		migrationNeeded = true
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	if migrationNeeded {
		migrate(db)
	}
	return db
}

func migrate(db *sql.DB) {
	sqlStmt := `
CREATE TABLE mails (file TEXT NOT NULL PRIMARY KEY, handled INTEGER);
    `
	_, err := db.Exec(sqlStmt)
	if err != nil {
		lg.LogeNoMail(err)
		log.Fatal("Failed to migrate database for mails.")
	}
	sqlStmt = `
CREATE TABLE events (file TEXT NOT NULL PRIMARY KEY, handled INTEGER);
    `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		lg.LogeNoMail(err)
		log.Fatal("Failed to migrate database for events.")
	}
}

func UpdateEmailState(db *sql.DB, file string, handled bool) error {
	sqlStmt, err := db.Prepare(`
REPLACE INTO mails(file, handled) VALUES(?, ?);
    `)
	if err != nil {
		return err
	}
	defer sqlStmt.Close()
	handledInt := 0
	if handled {
		handledInt = 1
	}
	sqlStmt.Exec(file, handledInt)
	return nil
}

func GetUnhandledMails(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
SELECT file FROM mails WHERE handled = 0;
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := make([]string, 0)
	for rows.Next() {
		var file string
		err = rows.Scan(&file)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func UpdateEventState(db *sql.DB, file string, handled bool) error {
	sqlStmt, err := db.Prepare(`
REPLACE INTO events(file, handled) VALUES(?, ?);
    `)
	if err != nil {
		return err
	}
	defer sqlStmt.Close()
	handledInt := 0
	if handled {
		handledInt = 1
	}
	sqlStmt.Exec(file, handledInt)
	return nil
}

func GetUnhandledEvents(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
SELECT file FROM events WHERE handled = 0;
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := make([]string, 0)
	for rows.Next() {
		var file string
		err = rows.Scan(&file)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}
