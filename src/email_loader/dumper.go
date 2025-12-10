// expose http endpoint for sendgrid, load email dumps, receive signals from docker_cron //
package email_loader

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.ibmgcloud.net/dth/inbound_parser/config"
	"github.ibmgcloud.net/dth/inbound_parser/handler"

	db "github.ibmgcloud.net/dth/inbound_parser/db"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func getBody(request *http.Request) ([]byte, error) {
	head, err := httputil.DumpRequest(request, false)
	if err != nil {
		return nil, err
	}
	bodyHead, err := httputil.DumpRequest(request, true)
	if err != nil {
		return nil, err
	}
	return bodyHead[len(head):], nil
}

func inboundHandler(response http.ResponseWriter, request *http.Request, cfg *glb.Config, noticedOutOfOffice *glb.NoticedOutOfOffice, idb *sql.DB) {
	token := request.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.SendgridToken)) != 1 {
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	// dump body
	body, err := getBody(request)
	if err != nil {
		lg.Loge(cfg, err)
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	// write file
	timestamp := strconv.Itoa(int(time.Now().UnixMicro()))
	dumpFile := "email_" + timestamp + ".dump"
	dumpFullPath := filepath.Join(cfg.DumpDir, dumpFile)
	if err := os.WriteFile(dumpFullPath, body, 0644); err != nil {
		lg.Loge(cfg, err)
		response.WriteHeader(http.StatusBadRequest)
		return
	}
	db.UpdateEmailState(idb, dumpFile, false)
	lg.Logf("received e-mail, dumped at '%s'\n", dumpFullPath)
	response.WriteHeader(http.StatusOK)

	// immediately parse request?
	if cfg.ParseRequests {
		lg.Logf("\n\n\n")
		if err := handler.HandleEmail(cfg, body, noticedOutOfOffice); err != nil {
			lg.Loge(cfg, err)
		} else {
			db.UpdateEmailState(idb, dumpFile, true)
		}
		lg.Logf("\n\n\n")
	}
}

func eventHandler(response http.ResponseWriter, request *http.Request, cfg *glb.Config, idb *sql.DB) {
	token := request.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.SendgridToken)) != 1 {
		lg.Logf("wrong token")
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	// dump body
	body, err := getBody(request)
	if err != nil {
		lg.Loge(cfg, err)
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	// write file
	timestamp := strconv.Itoa(int(time.Now().UnixMicro()))
	dumpFile := "event_" + timestamp + ".json"
	dumpFullPath := filepath.Join(cfg.DumpDir, dumpFile)
	if err := os.WriteFile(dumpFullPath, body, 0644); err != nil {
		lg.Loge(cfg, err)
		response.WriteHeader(http.StatusBadRequest)
		return
	}
	db.UpdateEventState(idb, dumpFile, false)
	lg.Logf("received event, dumped at '%s'\n", dumpFullPath)
	response.WriteHeader(http.StatusOK)

	lg.Logf("\n\n\n")
	if cfg.ParseRequests {
		if err := handler.HandleEvent(cfg, body); err != nil {
			lg.Loge(cfg, err)
			return
		} else {
			db.UpdateEventState(idb, dumpFile, true)
		}
	}
	lg.Logf("\n\n\n")
}

func StartEndlessRunner(cfg *glb.Config, noticedOutOfOffice *glb.NoticedOutOfOffice, idb *sql.DB) {
	// perform maintenance when receiving SIGHUP
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	var maintenance_mutex sync.Mutex

	go startDumper(cfg, &maintenance_mutex, noticedOutOfOffice, idb)
	for {
		<-sighup
		lg.Logf("received SIGHUP, acquiring mutex lock")
		maintenance_mutex.Lock()
		lg.Logf("lock engaged")

		config.Maintenance(cfg, noticedOutOfOffice)

		lg.Logf("unlocking mutex")
		maintenance_mutex.Unlock()
		lg.Logf("")
	}
}

func startDumper(cfg *glb.Config, maintenance_mutex *sync.Mutex, noticedOutOfOffice *glb.NoticedOutOfOffice, idb *sql.DB) {
	router := http.NewServeMux()
	router.HandleFunc("/inbound", func(w http.ResponseWriter, r *http.Request) {
		maintenance_mutex.Lock()
		inboundHandler(w, r, cfg, noticedOutOfOffice, idb)
		maintenance_mutex.Unlock()
	})
	if cfg.HandleEvents {
		router.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
			maintenance_mutex.Lock()
			eventHandler(w, r, cfg, idb)
			maintenance_mutex.Unlock()
		})
	}
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Port),
		Handler: router,
	}

	lg.Logf("Running dumping server on https://%s:%d using cert file '%s' and key file '%s' with parse_requests=%t, send_emails=%t and handle_events=%t\n\n",
		cfg.Domain, cfg.Port, cfg.SSLCert, cfg.SSLKey, cfg.ParseRequests, cfg.SendEmails, cfg.HandleEvents)
	if err := srv.ListenAndServeTLS(cfg.SSLCert, cfg.SSLKey); err != nil {
		lg.Loge(cfg, err)
	}
}
