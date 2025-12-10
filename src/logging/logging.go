// provide safe logs for docker compose //
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	"gopkg.in/gomail.v2"
)

var curLogFile *os.File

func SetupLogger() {
	log.SetFlags(log.Ldate | log.Ltime)
}

func CloseLogger() {
	if curLogFile != nil {
		Logf("closing log %s", curLogFile.Name())
		log.SetOutput(os.Stdout)
		curLogFile.Close()
	}
}

// needs to be called once at startup
func RotateLog(cfg *glb.Config) {
	CloseLogger()

	timestamp := strconv.Itoa(int(time.Now().UnixMicro()))
	filePath := filepath.Join(cfg.DumpDir, "log_"+timestamp+".log")
	curLogFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	multi := io.MultiWriter(curLogFile, os.Stdout)
	log.SetOutput(multi)

	Logf("rotated log file to %s", curLogFile.Name())
}

func Logf(format string, a ...any) string {
	str := strings.Trim(fmt.Sprintf(format, a...), "\r\n")
	str = strings.Replace(str, "\n", " | ", -1)
	str = strings.Replace(str, "\r", " | ", -1)
	log.Printf("%s\n", str)
	return str
}

func LogeNoMail(err error) string {
	Logf("Error:")
	return Logf(err.Error())
}

func Loge(cfg *glb.Config, err error) {
	Logf("Critical Error:")
	msg := Logf(err.Error())

	if cfg.CriticalMailTo == "" {
		return
	}
	// redundant, cut down implementation of email.send_mail.go
	m := gomail.NewMessage()
	Logf("Sending critical error mail from %s to %s", cfg.CriticalMailFrom, cfg.CriticalMailTo)
	m.SetAddressHeader("From", cfg.CriticalMailFrom, "")
	m.SetAddressHeader("To", cfg.CriticalMailTo, "")
	m.SetHeader("Subject", fmt.Sprintf("inbound_parser Error at %s", time.Now().String()))
	m.SetHeader("Auto-Submitted", "auto-generated")

	m.AddAlternative("text/plain", fmt.Sprintf("%s\nTraceback:\n%s", msg, string(debug.Stack())))

	dialer := gomail.NewDialer(cfg.SendEMailHost, cfg.SendEMailPort, "", "")
	if err := dialer.DialAndSend(m); err != nil {
		Logf(err.Error())
	}
}
