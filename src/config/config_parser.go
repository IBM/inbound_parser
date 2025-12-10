// parse and sanity-check yaml config //
package config

import (
	"html/template"
	"log"
	"net/mail"
	"os"

	"gopkg.in/yaml.v2"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	"github.ibmgcloud.net/dth/inbound_parser/jira_actor"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func validateServiceDesk(srd *glb.ServiceDesk) {
	if srd.ProjectKey == "" {
		log.Fatal("project_key must be defined for every serviceDesk")
	}
	var err error
	srd.Id, err = jira_actor.GetServiceDeskId(srd.ProjectKey, srd.JiraInstall.Client)
	if err != nil {
		log.Fatal(err)
	}

	if srd.RequestType == "" {
		log.Fatalf("request_type needs to be defined in serviceDesk %s\n", srd.ProjectKey)
	}
	srd.RequestTypeId, err = jira_actor.GetRequestTypeId(srd.RequestType, srd.Id, srd.JiraInstall.Client)
	if err != nil {
		log.Fatal(err)
	}
	// RequestPostfix is optional

	srd.OnlyCreateEventRequests = srd.CreateEventRequests && len(srd.JiraInstall.Emails) == 0 && len(srd.Emails) == 0

	if srd.OnlyCreateEventRequests {
		if srd.RequestPostfix != "" || srd.ReplyEmailName != "" || srd.RequestCreationEmailTextPlainPath != "" || srd.ReplyAboveThis != "" {
			log.Fatalf("request_postfix, reply_email_name, request_creation_email_text_plain_path and reply_above_this shall not be define for servicedesk %s, which is exclusively used for event requests",
				srd.ProjectKey)
		}
		return
	}

	if len(srd.Emails) == 0 && len(srd.JiraInstall.Emails) == 0 {
		log.Fatalf("emails needs to be defined in serviceDesk %s , which isn't used for event request creation, or in jira install %s\n", srd.ProjectKey, srd.JiraInstall.URL)
	}
	for _, email := range srd.Emails {
		// check no email is used twice
		otherServiceDesk := GetServiceDeskFromMail(srd.JiraInstall.Cfg, email)
		if otherServiceDesk != srd {
			log.Fatalf("email %s can't be used for both %s and %s\n", email, srd.ProjectKey, otherServiceDesk.ProjectKey)
		}
		otherJiraInstall := GetJiraInstallFromMail(srd.JiraInstall.Cfg, email)
		if otherJiraInstall != nil {
			log.Fatalf("email %s can't be used for both serviceDesk %s and jira install %s\n", email, srd.ProjectKey, otherJiraInstall.URL)
		}
	}
	if len(srd.Emails) == 0 {
		srd.ReplyAddress = &mail.Address{Name: srd.ReplyEmailName, Address: srd.JiraInstall.Emails[0]}
	} else {
		srd.ReplyAddress = &mail.Address{Name: srd.ReplyEmailName, Address: srd.Emails[0]}
	}

	// requests cannot be created when there is no serviceDesk email
	if srd.JiraInstall.Cfg.SendEmails && len(srd.Emails) != 0 {
		// RequestCreationEmailTextPlainPath
		if fileInfo, err := os.Stat(srd.RequestCreationEmailTextPlainPath); err != nil || fileInfo.IsDir() {
			log.Fatalf("'%s' specified by request_creation_email_text_plain_path in the config doesn't point to a file for servicedesk %s\n", srd.RequestCreationEmailTextPlainPath, srd.Id)
		}
		var err error
		srd.RequestCreationEmailTextPlainTemplate, err = template.ParseFiles(srd.RequestCreationEmailTextPlainPath)
		if err != nil {
			log.Fatal(err)
		}
	}
	// DontCommentRequestStatus is optional
}

func validateJiraInstall(jiraInstall *glb.JiraInstall) {
	if jiraInstall.Token == "" {
		log.Fatalf("token needs to be defined for every jira install\n")
	}
	if jiraInstall.URL == "" {
		log.Fatalf("url needs to be defined for every jira install\n")
	}

	var err error
	jiraInstall.Client, err = jira_actor.GetJiraClient(jiraInstall.URL, jiraInstall.Token)
	if err != nil {
		log.Fatal(err)
	}
	if jiraInstall.AdminToken != "" {
		jiraInstall.AdminClient, err = jira_actor.GetJiraClient(jiraInstall.URL, jiraInstall.AdminToken)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		lg.Logf("warning: no admin token has been provided, customer creation is disabled")
	}

	for _, email := range jiraInstall.Emails {
		// check no email is used twice
		// this first check is redundant as the servicedesk already check for jira installs with the same address
		otherServiceDesk := GetServiceDeskFromMail(jiraInstall.Cfg, email)
		if otherServiceDesk != nil {
			log.Fatalf("email %s can't be used for both %s and %s\n", email, jiraInstall.URL, otherServiceDesk.ProjectKey)
		}
		otherJiraInstall := GetJiraInstallFromMail(jiraInstall.Cfg, email)
		if otherJiraInstall != jiraInstall {
			log.Fatalf("email %s can't be used for both jira install %s and jira install %s\n", email, jiraInstall.URL, otherJiraInstall.URL)
		}
	}
	if len(jiraInstall.Emails) != 0 {
		// RejectedMailSubject may be left blank

		jiraInstall.RejectedMailTemplate, err = template.ParseFiles(jiraInstall.RejectedMailPath)
		if err != nil {
			log.Fatal(err)
		}
		jiraInstall.ReplyAddress = &mail.Address{Name: jiraInstall.ReplyEmailName, Address: jiraInstall.Emails[0]}
	}

	for _, srd := range jiraInstall.ServiceDesks {
		srd.JiraInstall = jiraInstall
		validateServiceDesk(srd)
	}
}

func validateConfig(cfg *glb.Config) {
	if (cfg.CriticalMailTo == "") != (cfg.CriticalMailFrom == "") {
		log.Fatal("either both critical_mail_to and critical_mail_from need to be defined or neither")
	}

	if !cfg.DumpRequests && !cfg.ParseRequests {
		log.Fatal("one or both of (dump_dir, parse_requests) need to be set to true in config.yaml")
	}
	if cfg.CheckMalware {
		if cfg.ClamAVScandir == "" {
			log.Fatal("clamav_scandir needs to be defined")
		}
	}
	if fileInfo, err := os.Stat(cfg.DumpDir); err != nil || !fileInfo.IsDir() {
		log.Fatalf("'%s' specified by dump_dir in the config doesn't point to a directory\n", cfg.DumpDir)
	}

	if cfg.MaxSpamScore <= 0 {
		log.Fatal("spam score needs to be defined and bigger than 0")
	}

	if cfg.DumpRequests {
		if cfg.Port == 0 {
			log.Fatal("port needs to be defined")
		}
		if cfg.Domain == "" {
			log.Fatal("domain needs to be defined")
		}
		// certs get checked by golang http
	}

	// ignore jira_install in debug parse mode
	if cfg.ParseRequests && !cfg.DebugParseOnly {
		for _, jiraInstall := range cfg.JiraInstalls {
			jiraInstall.Cfg = cfg
			validateJiraInstall(jiraInstall)
		}
		if cfg.MaxParticipants == 0 {
			log.Fatal("max_participants must be defined")
		}
	}

	if cfg.SendEmails {
		if cfg.SendEMailHost == "" {
			log.Fatal("send_email_host needs to be defined")
		}
		if cfg.SendEMailPort == 0 {
			log.Fatal("send_email_port needs to be defined")
		}
	}

	if cfg.HandleEvents {
		// HandleEventsUsername may be left blank
		for _, jiraInstall := range cfg.JiraInstalls {
			for _, srd := range jiraInstall.ServiceDesks {
				if srd.CreateEventRequests {
					if cfg.HandleEventsSrd != nil {
						log.Fatalf("%s and at least one other servicedesk have create_event_requests set, only one is allowed to\n", srd.ProjectKey)
					}
					cfg.HandleEventsSrd = srd
				}
			}
		}
		if cfg.HandleEventsSrd == nil {
			log.Fatal("At least one servicedesk needs to have create_event_requests set\n")
		}
		lg.Logf("Using servicedesk %s for event request creation", cfg.HandleEventsSrd.ProjectKey)
	}
}

func GetCfg() *glb.Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		lg.LogeNoMail(err)
		log.Fatalf("Couldn't read config file '%s'\n", configPath)
	}
	var cfg glb.Config
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		lg.LogeNoMail(err)
		log.Fatal("Failed to parse yaml config file.")
	}
	validateConfig(&cfg)
	lg.Logf("loaded and validated config")

	return &cfg
}
