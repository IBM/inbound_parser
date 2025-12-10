// everything required to be done regularly -> started with docker_cron //
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func getTimestamp(fileName string) time.Time {
	fileName = strings.ReplaceAll(fileName, "email_", "")
	fileName = strings.ReplaceAll(fileName, "event_", "")
	fileName = strings.ReplaceAll(fileName, "log_", "")
	fileName = strings.ReplaceAll(fileName, ".json", "")
	fileName = strings.ReplaceAll(fileName, ".dump", "")
	fileName = strings.ReplaceAll(fileName, ".log", "")
	timestampInt, err := strconv.Atoi(fileName)
	if err != nil {
		lg.Logf("failed to get timestamp of %s\n", fileName)
		return time.Now()
	}
	return time.UnixMicro(int64(timestampInt))
}

// complying with privacy laws
func deleteOldEmails(cfg *glb.Config) {
	if cfg.EmailKeepDays == 0 {
		lg.Logf("don't delete any emails, as defined in email_keep_days config")
		return
	}
	timeCutOff := time.Now().AddDate(0, 0, -cfg.EmailKeepDays)

	files, err := os.ReadDir(cfg.DumpDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if getTimestamp(file.Name()).Before(timeCutOff) {
			path := cfg.DumpDir + "/" + file.Name()
			lg.Logf("deleting %s\n", path)
			err := os.Remove(path)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	lg.Logf("deletion done")
}

func deleteNoticedOutOfOffice(noticedOutOfOffice *glb.NoticedOutOfOffice) {
	lg.Logf("deleting noticed out of office entries")
	for oneNoticedOutOfOffice := range *noticedOutOfOffice {
		lg.Logf("deleting %s\n", oneNoticedOutOfOffice)
		delete(*noticedOutOfOffice, oneNoticedOutOfOffice)
	}
	lg.Logf("deletion done")
}

func Maintenance(cfg *glb.Config, noticedOutOfOffice *glb.NoticedOutOfOffice) {
	lg.Logf("performing maintenance")
	deleteOldEmails(cfg)
	deleteNoticedOutOfOffice(noticedOutOfOffice)
	lg.RotateLog(cfg)
	lg.Logf("maintenance completed")
}
