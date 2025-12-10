// expose http endpoint for sendgrid, load email dumps, receive signals from docker_cron //
package email_loader

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.ibmgcloud.net/dth/inbound_parser/handler"

	db "github.ibmgcloud.net/dth/inbound_parser/db"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func LoadAllRequestDumps(cfg *glb.Config, noticedOutOfOffice *glb.NoticedOutOfOffice) {
	lg.Logf("Loading Email Dumps with send_emails=%t\n\n", cfg.SendEmails)
	files, err := os.ReadDir(cfg.DumpDir)
	if err != nil {
		lg.LogeNoMail(err)
		log.Fatalf("")
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".dump") {
			continue
		}

		lg.Logf("\n\n\n")
		path := filepath.Join(cfg.DumpDir, file.Name())

		lg.Logf("reading email dumped at '%s'\n", path)
		body, err := os.ReadFile(path)
		if err != nil {
			lg.LogeNoMail(err)
			log.Fatalf("")
		}
		if err := handler.HandleEmail(cfg, body, noticedOutOfOffice); err != nil {
			lg.LogeNoMail(err)
			log.Fatalf("")
		}
		lg.Logf("\n\n\n")
	}
}

func LoadUnhandledDumps(cfg *glb.Config, noticedOutOfOffice *glb.NoticedOutOfOffice, idb *sql.DB) {
	lg.Logf("Loading unhandled Dumps with send_emails=%t\n\n", cfg.SendEmails)
	mails, err := db.GetUnhandledMails(idb)
	if err != nil {
		lg.Loge(cfg, err)
	}
	for _, dumpFile := range mails {
		lg.Logf("\n\n\n")
		path := filepath.Join(cfg.DumpDir, dumpFile)

		lg.Logf("reading email dumped at '%s'\n", path)
		body, err := os.ReadFile(path)
		if err != nil {
			lg.Loge(cfg, err)
		} else {
			if err := handler.HandleEmail(cfg, body, noticedOutOfOffice); err != nil {
				lg.Loge(cfg, err)
			} else {
				db.UpdateEmailState(idb, dumpFile, true)
			}
		}
		lg.Logf("\n\n\n")
	}

	events, err := db.GetUnhandledEvents(idb)
	if err != nil {
		lg.Loge(cfg, err)
	}
	for _, dumpFile := range events {
		lg.Logf("\n\n\n")
		path := filepath.Join(cfg.DumpDir, dumpFile)

		lg.Logf("reading event dumped at '%s'\n", path)
		body, err := os.ReadFile(path)
		if err != nil {
			lg.Loge(cfg, err)
		} else {
			if err := handler.HandleEvent(cfg, body); err != nil {
				lg.Loge(cfg, err)
				return
			} else {
				db.UpdateEventState(idb, dumpFile, true)
			}
		}
		lg.Logf("\n\n\n")
	}
	lg.Logf("Finished Loading unhandled Dumps")
}
