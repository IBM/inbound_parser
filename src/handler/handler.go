// central high level instructions, what to do with incoming mail //
// this code shall be as easily changeable as possible //
package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"github.ibmgcloud.net/dth/inbound_parser/config"
	"github.ibmgcloud.net/dth/inbound_parser/email"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func HandleEmail(cfg *glb.Config, emailBody []byte, noticedOutOfOffice *glb.NoticedOutOfOffice) error {
	ehp, err := prepareEmailHandling(cfg, emailBody)
	if err != nil {
		return err
	}

	if cfg.DebugParseOnly {
		// this is a debug flag so we can savely print 'untrusted' input to stdout
		// this flag is never to be used in production
		jsonString, err := json.MarshalIndent(ehp.Email, "", "  ")
		if err != nil {
			return err
		}
		log.Printf("%s\n\n\n\n", jsonString)
		return nil
	}

	if ehp.Email.IsMalware {
		lg.Logf("this is malware")
		lg.Logf("ignore")
		return nil
	}

	if config.GetServiceDeskFromMail(cfg, ehp.Email.From.Address) != nil {
		lg.Logf("email is from an address assigned to a serviceDesk")
		lg.Logf("aborting to prevent endless loop")
		return nil
	}

	if config.GetJiraInstallFromMail(cfg, ehp.Email.From.Address) != nil {
		lg.Logf("email is from an address assigned to a jira install")
		lg.Logf("aborting to prevent endless loop")
		return nil
	}

	if ehp.Whitelisted {
		lg.Logf("sender address is whitelisted, ignore spam score and auto reply status")
	} else {
		lg.Logf("sender address is not whitelisted")

		if ehp.Email.SpamScore >= cfg.MaxSpamScore {
			lg.Logf("spam score is too high")
			lg.Logf("ignore")
			return nil
		}

		if ehp.Email.IsAutoReply {
			lg.Logf("email is an auto reply")
			_, found := (*noticedOutOfOffice)[ehp.Email.From.Address]
			if found {
				lg.Logf("%s has already been handled\n", email.FormatAddr(ehp.Email.From))
				lg.Logf("ignore")
				return nil
			}
			if ehp.Request == nil {
				lg.Logf("auto-replies don't get used to create new requests")
				lg.Logf("ignore")
				return nil
			}
			lg.Logf("%s has not already been handled\n", email.FormatAddr(ehp.Email.From))
			(*noticedOutOfOffice)[ehp.Email.From.Address] = struct{}{}
		}
	}

	if ehp.JiraInstall == nil {
		lg.Logf("addressee isn't assigned to any serviceDesk or jira install")
		lg.Logf("ignore")
		return nil
	}

	lg.Logf("addressee, a Cc or Bcc refers to serviceDesk email or jira install")
	if ehp.Request == nil {
		lg.Logf("subject doesn't contain valid issue key")
		if ehp.ServiceDesk == nil {
			lg.Logf("the email went to a jira install, not a serviceDesk; the inbound_parser doesn't know where to create the new request")
			lg.Logf("send error mail to customer")
			email.SendWrongAddressErrorEmail(ehp.JiraInstall, ehp.Email)
			return nil
		}
		createdRequest, err := createRequestFromEmail(ehp.ServiceDesk, ehp.SenderJiraUsername, true, ehp.Email, ehp.DontReplyTo)
		if err != nil {
			return err
		}
		// jira already sends request creation reply email when user is known or got created
		if ehp.SenderJiraUsername == "" {
			lg.Logf("user without jira account")
			if ehp.DontReplyTo {
				lg.Logf("email sender is in don't reply list")
			} else {
				lg.Logf("email sender is not in don't reply list")
				err = email.SendRequestCreatedEmail(ehp.ServiceDesk, ehp.Email, createdRequest)
				if err != nil {
					return err
				}
			}
		}
	} else {
		lg.Logf("subject contains valid issue key in serviceDesk's jira install or addressed jira install")
		if ehp.DontComment {
			lg.Logf("the status '%s' is not to be commented", ehp.Request.Status)
			lg.Logf("ignore")
			return nil
		}
		// always create the request as the request id is valid
		err := createCommentFromEmail(ehp.RequestServiceDesk, ehp.Request, ehp.SenderJiraUsername, ehp.Email, ehp.DontReplyTo)
		if err != nil {
			return err
		}
		// don't send reply email <- jira already does as this is probably a reply to a mail from jira
	}

	return nil
}

func HandleEvent(cfg *glb.Config, eventBody []byte) error {
	lg.Logf("handling event")
	events, err := getEvents(cfg, eventBody)
	if err != nil {
		return err
	}

	for _, event := range events {
		summary, description, files := getEvent(event)
		lg.Logf(string(event))
		lg.Logf(summary)
		lg.Logf(description)
		lg.Logf("handled with: %s\n", cfg.HandleEventsSrd.ProjectKey)
		_, err := createRequestFromEvent(cfg, summary, description, files)
		if err != nil {
			return err
		}
	}
	return nil
}

func getEvent(eventJson []byte) (string, string, []glb.File) {
	sendgridSummary, sendgridDescription, sendgridFiles, isSendgridEvent := getSendgridEvent(eventJson)
	if isSendgridEvent {
		lg.Logf("is Sendgrid event")
		return sendgridSummary, sendgridDescription, sendgridFiles
	}
	githubSummary, githubDescription, githubFiles, isGitHubDescription := getGitHubMonitorEvent(eventJson)
	if isGitHubDescription {
		lg.Logf("is GitHub event")
		return githubSummary, githubDescription, githubFiles
	}
	jiraCveSummary, jiraCveDescription, jiraCveFiles, isJiraCveEvent := getJiraCveEvent(eventJson)
	if isJiraCveEvent {
		lg.Logf("is Jira CVE event")
		return jiraCveSummary, jiraCveDescription, jiraCveFiles
	}
	sysdigSummary, sysdigDescription, sysdigFiles, isSysdigEvent := getSysdigEvent(eventJson)
	if isSysdigEvent {
		lg.Logf("is Sysdig event")
		return sysdigSummary, sysdigDescription, sysdigFiles
	}

	return "Unknown Event Type", string(eventJson), make([]glb.File, 0)
}

// return summary, description, attachments and true iff this is a sendgrid event
func getSendgridEvent(eventJson []byte) (string, string, []glb.File, bool) {
	type SendgridEvent struct {
		Email     string `json:"email"`
		EventType string `json:"event"`
	}
	var sendgridEvent SendgridEvent
	err := json.Unmarshal(eventJson, &sendgridEvent)
	if err != nil {
		return "", "", make([]glb.File, 0), false
	}
	if sendgridEvent.Email == "" {
		return "", "", make([]glb.File, 0), false
	}
	if sendgridEvent.EventType == "" {
		return "", "", make([]glb.File, 0), false
	}

	return fmt.Sprintf("Sendgrid: %s %s", sendgridEvent.EventType, sendgridEvent.Email), string(eventJson), make([]glb.File, 0), true
}

// return summary, description, attachments and true iff this is a github monitor event
func getGitHubMonitorEvent(eventJson []byte) (string, string, []glb.File, bool) {
	type GitHubEvent struct {
		Summary   string `json:"summary"`
		EventBody string `json:"event_body"`
		Type      string `json:"type"`
	}
	var githubEvent GitHubEvent
	err := json.Unmarshal(eventJson, &githubEvent)
	if err != nil {
		return "", "", make([]glb.File, 0), false
	}
	if githubEvent.Type != "github_monitor" {
		return "", "", make([]glb.File, 0), false
	}
	if githubEvent.EventBody == "" {
		return "", "", make([]glb.File, 0), false
	}

	return fmt.Sprintf("GitHub Monitor: %s", githubEvent.Summary), githubEvent.EventBody, make([]glb.File, 0), true
}

// return summary, description, attachments and true iff this is a Jira CVE event
func getJiraCveEvent(eventJson []byte) (string, string, []glb.File, bool) {
	type B64File struct {
		Name    string `json:"name"`
		B64Data string `json:"b64_data"`
	}
	type JiraCveEvent struct {
		Summary   string    `json:"subject"`
		EventBody string    `json:"body"`
		Files     []B64File `json:"files"`
		Type      string    `json:"type_field"`
	}
	var jiraCveEvent JiraCveEvent
	err := json.Unmarshal(eventJson, &jiraCveEvent)
	if err != nil {
		return "", "", make([]glb.File, 0), false
	}
	if jiraCveEvent.Type != "cve_scanner" {
		return "", "", make([]glb.File, 0), false
	}
	if jiraCveEvent.EventBody == "" {
		return "", "", make([]glb.File, 0), false
	}

	var files []glb.File
	for _, file := range jiraCveEvent.Files {
		fileBytes, err := base64.StdEncoding.DecodeString(file.B64Data)
		if err != nil {
			return "", "", make([]glb.File, 0), false
		}
		files = append(files, glb.File{Name: file.Name, Bytes: fileBytes})
	}

	return fmt.Sprintf("Jira CVE: %s", jiraCveEvent.Summary), jiraCveEvent.EventBody, files, true
}

// return summary, description, attachments and true iff this is a sysdig event
func getSysdigEvent(eventJson []byte) (string, string, []glb.File, bool) {
	type SysdigEvent struct {
		Summary   string `json:"summary"`
		EventBody string `json:"event_body"`
	}
	var sysdigEvent SysdigEvent
	err := json.Unmarshal(eventJson, &sysdigEvent)
	if err != nil {
		return "", "", make([]glb.File, 0), false
	}
	if sysdigEvent.EventBody == "" {
		return "", "", make([]glb.File, 0), false
	}

	return fmt.Sprintf("Sysdig: %s", sysdigEvent.Summary), sysdigEvent.EventBody, make([]glb.File, 0), true
}
