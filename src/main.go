// entry point //
package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	config "github.ibmgcloud.net/dth/inbound_parser/config"
	db "github.ibmgcloud.net/dth/inbound_parser/db"
	"github.ibmgcloud.net/dth/inbound_parser/email_loader"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
	"github.ibmgcloud.net/dth/inbound_parser/malware_detection"
)

func main() {
	fmt.Print(`
  ==== ======   ====       ====
  ==== ======== ====       ====
   ==   ==   ==   ===     ===  
   ==   ==   ==   ====   ====  
   ==   ======    == == == ==  
   ==   ======    == == == ==  
   ==   ==   ==   ==  ===  ==  
   ==   ==   ==   ==  ===  ==  
  ==== ======== ====   =   ====
  ==== ======   ====       ====

*********************************
*  ==============               *
*  inbound_parser               *
*  ==============               *
*                               *
*  Maintained by                *
*  Christopher Besch            *
*  <christopher.besch@ibm.com>  *
*********************************

`)
	// ignore sighup until system has booted
	signal.Ignore(syscall.SIGHUP)
	lg.SetupLogger()
	defer lg.CloseLogger()

	cfg := config.GetCfg()
	lg.RotateLog(cfg)
	lg.Loge(cfg, errors.New("inbound_parser booting up"))

	idb := db.GetDb()
	defer idb.Close()

	malware_detection.WaitForClamAV(cfg)

	if cfg.PrintLicenses {
		printLicenses()
	}

	// when don't reply to email is received to a request, create a comment only once
	noticedOutOfOffice := glb.NoticedOutOfOffice{}

	// immediately parsing requests gets performed in startDumper if required
	if cfg.DumpRequests {
		email_loader.LoadUnhandledDumps(cfg, &noticedOutOfOffice, idb)
		email_loader.StartEndlessRunner(cfg, &noticedOutOfOffice, idb)
		os.Exit(0)
	}
	if cfg.ParseRequests {
		email_loader.LoadAllRequestDumps(cfg, &noticedOutOfOffice)
		os.Exit(0)
	}
}
