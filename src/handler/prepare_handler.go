// extract everything possible from parsed email -> everything required for handler.go //
package handler

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.ibmgcloud.net/dth/inbound_parser/config"
	"github.ibmgcloud.net/dth/inbound_parser/email"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	"github.ibmgcloud.net/dth/inbound_parser/jira_actor"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func getRequestFromEmail(jiraInstall *glb.JiraInstall, emailSubject string) (*glb.Request, *glb.ServiceDesk, error) {
	re := regexp.MustCompile(`[A-Z]+-\d+`)
	matches := re.FindAllString(emailSubject, -1)

	for _, match := range matches {
		request, err := jira_actor.GetRequest(match, jiraInstall.Client)
		if err != nil {
			return nil, nil, err
		}
		if request == nil {
			lg.Logf("attemting to comment request %s that doesn't exist in jira install %s or is an issue\n", match, jiraInstall.URL)
			continue
		}
		srd := config.GetServiceDeskFromId(jiraInstall.Cfg, request.ServiceDeskId)
		if srd == nil {
			lg.Logf("attemting to comment request %s of serviceDesk that hasn't been registered for jira install %s\n", match, jiraInstall.URL)
			continue
		}
		return request, srd, nil
	}
	return nil, nil, nil
}

func notToReplyTo(cfg *glb.Config, address string) bool {
	for _, dontReplyToEmail := range cfg.DontReplyToEmails {
		if dontReplyToEmail == address || strings.ToLower(dontReplyToEmail) == strings.ToLower(address) {
			return true
		}
	}
	return false
}

func whitelisted(cfg *glb.Config, address string) bool {
	for _, whitelistedAddress := range cfg.EmailWhitelist {
		if whitelistedAddress == address || strings.ToLower(whitelistedAddress) == strings.ToLower(address) {
			return true
		}
	}
	return false
}

func prepareEmailHandling(cfg *glb.Config, emailBody []byte) (*glb.EmailHandlingParam, error) {
	lg.Logf("loading email handling params")
	ehp := glb.EmailHandlingParam{}
	var err error

	ehp.Email, err = email.GetParsedEmail(emailBody, cfg)
	if err != nil {
		return nil, err
	}
	lg.Logf(email.GetEmailStatsStr(ehp.Email))

	ehp.DontReplyTo = notToReplyTo(cfg, ehp.Email.From.Address)
	ehp.Whitelisted = whitelisted(cfg, ehp.Email.From.Address)

	// you could optimize these two loops into a single on
	// but like this the behavior is defined: when there is a serviceDesk addressed, use that
	// otherwise check if jira installs are addressed
	// is email.To or an email.Cc or an email.Bcc referring to a serviceDesk?
	for _, to := range append(append(ehp.Email.To, ehp.Email.Cc[:]...), ehp.Email.Bcc[:]...) {
		ehp.ServiceDesk = config.GetServiceDeskFromMail(cfg, to.Address)
		if ehp.ServiceDesk != nil {
			break
		}
	}
	if ehp.ServiceDesk != nil {
		ehp.JiraInstall = ehp.ServiceDesk.JiraInstall
	} else {
		// is email.To or an email.Cc or an email.Bcc referring to a jira install?
		for _, to := range append(append(ehp.Email.To, ehp.Email.Cc[:]...), ehp.Email.Bcc[:]...) {
			ehp.JiraInstall = config.GetJiraInstallFromMail(cfg, to.Address)
			if ehp.JiraInstall != nil {
				break
			}
		}
		if ehp.JiraInstall == nil {
			// email went to address not specified anywhere, probably to be ignored
			return &ehp, nil
		}
	}

	ehp.SenderJiraUsername, err = createAndGetJiraUsername(ehp.Email.From, ehp.JiraInstall)
	if err != nil {
		return nil, err
	}

	// is a request referenced in the email's subject
	ehp.Request, ehp.RequestServiceDesk, err = getRequestFromEmail(ehp.JiraInstall, ehp.Email.Subject)
	if err != nil {
		return nil, err
	}

	ehp.DontComment = false
	if ehp.Request != nil {
		for _, dontCommentRequestStatus := range ehp.RequestServiceDesk.DontCommentRequestStatus {
			if dontCommentRequestStatus == ehp.Request.Status {
				ehp.DontComment = true
				break
			}
		}
	}

	return &ehp, nil
}

func getEvents(cfg *glb.Config, eventBody []byte) ([][]byte, error) {
	lg.Logf("parse event json")
	var fullJsons []interface{}
	err := json.Unmarshal(eventBody, &fullJsons)
	if err != nil {
		return nil, err
	}

	lg.Logf("prettify event json")
	var events [][]byte
	for _, fullJson := range fullJsons {
		prettyBody, err := json.MarshalIndent(&fullJson, "", "    ")
		if err != nil {
			return nil, err
		}

		events = append(events, prettyBody)
	}
	return events, nil
}
