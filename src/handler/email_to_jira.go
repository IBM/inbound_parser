// high level jira interaction -> take data from email, convert into jira request/comment/... //
package handler

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.ibmgcloud.net/dth/inbound_parser/config"
	"github.ibmgcloud.net/dth/inbound_parser/email"
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	"github.ibmgcloud.net/dth/inbound_parser/jira_actor"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func createSummary(srd *glb.ServiceDesk, mail *glb.Email) string {
	summary := mail.Subject
	if srd.RequestPostfix != "" {
		summary += " --- " + srd.RequestPostfix
	}
	summary = strings.TrimSpace(summary)
	summary = strings.ReplaceAll(summary, "\n", " ")
	return summary
}

func createDescription(srd *glb.ServiceDesk, mail *glb.Email, knownUser bool) string {
	message := mail.TextBody
	if srd.ReplyAboveThis != "" {
		message = strings.Split(message, srd.ReplyAboveThis)[0]
	}

	return fmt.Sprintf("Received via mail\n\n%s\n\n%s", email.GetEmailStatsStr(mail), message)
}

func addAddresseesAsParticipants(srd *glb.ServiceDesk, requestKey string, mail *glb.Email, dontReplyTo bool, requestReporter string, requestAssignee string) error {
	addedParticipants := uint(0)
	for _, address := range append(append(mail.Cc, mail.To[:]...), mail.Bcc[:]...) {
		if addedParticipants >= srd.JiraInstall.Cfg.MaxParticipants {
			lg.Logf("Stopping addition of more participants")
			break
		}
		// skip when to address refers to jira servicedesk
		if config.GetServiceDeskFromMail(srd.JiraInstall.Cfg, address.Address) != nil {
			continue
		}
		// skip when to address refers to jira install
		if config.GetJiraInstallFromMail(srd.JiraInstall.Cfg, address.Address) != nil {
			continue
		}
		user, err := createAndGetJiraUsername(address, srd.JiraInstall)
		if err != nil {
			return err
		}
		if user != "" {
			if user == requestReporter {
				lg.Logf("don't add reporter of request as participant")
			} else if user == requestAssignee {
				lg.Logf("don't add assignee of request as participant")
			} else {
				lg.Logf("adding %s, aka %s as participant\n", email.FormatAddr(address), user)
				err := jira_actor.AddParticipant(requestKey, user, srd.JiraInstall.Client)
				if err != nil {
					return err
				}
				addedParticipants++
			}
		}
	}
	return nil
}

func createRequestFromEmail(srd *glb.ServiceDesk, reporterUsername string, tryAnonymous bool, mail *glb.Email, dontReplyTo bool) (*glb.Request, error) {
	knownUser := reporterUsername != ""
	lg.Logf("create request, known user: %t\n", knownUser)
	summary := createSummary(srd, mail)
	description := createDescription(srd, mail, knownUser)
	requestKey, err := jira_actor.CreateRequest(summary, description, reporterUsername, srd.RequestTypeId, srd.Id, tryAnonymous, srd.JiraInstall.Client)
	if err != nil {
		return nil, err
	}
	lg.Logf("created new request: %s\n", requestKey)
	if len(mail.Files) != 0 {
		lg.Logf("uploading attachments")
		jira_actor.CreateComment("", mail.Files, requestKey, srd.Id, srd.JiraInstall.Client)
	}

	// assignee is never set right after creation
	err = addAddresseesAsParticipants(srd, requestKey, mail, dontReplyTo, reporterUsername, "")
	if err != nil {
		return nil, err
	}
	return jira_actor.GetRequest(requestKey, srd.JiraInstall.Client)
}

func createRequestFromEvent(cfg *glb.Config, summary string, description string, files []glb.File) (*glb.Request, error) {
	lg.Logf("create request from event")
	requestKey, err := jira_actor.CreateRequest(
		summary,
		description,
		cfg.HandleEventsUsername,
		cfg.HandleEventsSrd.RequestTypeId,
		cfg.HandleEventsSrd.Id,
		false,
		cfg.HandleEventsSrd.JiraInstall.Client,
	)
	if err != nil {
		return nil, err
	}
	lg.Logf("created new request: %s\n", requestKey)
	if len(files) != 0 {
		lg.Logf("uploading attachments")
		jira_actor.CreateComment("", files, requestKey, cfg.HandleEventsSrd.Id, cfg.HandleEventsSrd.JiraInstall.Client)
	}
	return jira_actor.GetRequest(requestKey, cfg.HandleEventsSrd.JiraInstall.Client)
}

func createCommentFromEmail(srd *glb.ServiceDesk, request *glb.Request, commenterUsername string, mail *glb.Email, dontReplyTo bool) error {
	knownUser := commenterUsername != ""
	lg.Logf("create comment, known user: %t\n", knownUser)
	description := createDescription(srd, mail, knownUser)
	err := jira_actor.CreateComment(description, mail.Files, request.IssueKey, srd.Id, srd.JiraInstall.Client)
	if err != nil {
		return err
	}
	lg.Logf("created new comment for %s\n", request.IssueKey)

	if commenterUsername != "" {
		if commenterUsername == request.Reporter {
			lg.Logf("don't add reporter of request as participant")
		} else if commenterUsername == request.Assignee {
			lg.Logf("don't add assignee of request as participant")
		} else {
			lg.Logf("adding commenter as participant")
			err := jira_actor.AddParticipant(request.IssueKey, commenterUsername, srd.JiraInstall.Client)
			if err != nil {
				return err
			}
		}
	}
	return addAddresseesAsParticipants(srd, request.IssueKey, mail, dontReplyTo, request.Reporter, request.Assignee)
}

func createAndGetJiraUsername(address *mail.Address, jiraInstall *glb.JiraInstall) (string, error) {
	// get with normal address
	user, err := jira_actor.GetJiraUsername(address.Address, address.Name, jiraInstall.Client)
	if err != nil {
		return "", err
	}
	if user != "" {
		return user, nil
	}

	// get with lower case address
	user, err = jira_actor.GetJiraUsername(strings.ToLower(address.Address), address.Name, jiraInstall.Client)
	if err != nil {
		return "", err
	}
	if user != "" {
		return user, nil
	}

	// create user
	if jiraInstall.AdminToken == "" {
		lg.Logf("admin token not defined, can't create customer")
		return "", nil
	}
	if notToReplyTo(jiraInstall.Cfg, address.Address) {
		lg.Logf("don't create user not to reply to")
		return "", nil
	}

	err = jira_actor.CreateCustomer(address.Address, address.Name, jiraInstall.AdminClient)
	if err != nil {
		lg.LogeNoMail(err)
		lg.Logf("ignore this error")
		return "", nil
	}
	user, err = jira_actor.GetJiraUsername(address.Address, address.Name, jiraInstall.Client)
	if err != nil {
		// this should never happen
		return "", err
	}
	if user == "" {
		return "", errors.New(fmt.Sprintf("attempting customer creation of %s but didn't find that user afterwards\n", email.FormatAddr(address)))
	}
	return user, nil
}
