// send reply emails //
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/mail"

	"gopkg.in/gomail.v2"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func sendMail(cfg *glb.Config, from *mail.Address, to *mail.Address, subject string, body string) error {
	m := gomail.NewMessage()
	m.SetAddressHeader("From", from.Address, from.Name)
	m.SetAddressHeader("To", to.Address, to.Name)
	m.SetHeader("Subject", subject)
	m.SetHeader("Auto-Submitted", "auto-generated")

	m.AddAlternative("text/plain", body)

	dialer := gomail.NewPlainDialer(cfg.SendEMailHost, cfg.SendEMailPort, "", "")
	lg.Logf("sending mail at %s from %s: %s\n", FormatAddr(to), FormatAddr(from), subject)
	if err := dialer.DialAndSend(m); err != nil {
		return err
	}
	return nil
}

func sendReplyEmail(cfg *glb.Config, email *glb.Email, template *template.Template, templateData any, subject string, from *mail.Address) error {
	var buffer bytes.Buffer
	err := template.Execute(&buffer, templateData)
	if err != nil {
		return err
	}

	body := buffer.String() + "\n" + getQuotedTextBody(email)
	err = sendMail(cfg, from, email.From, subject, body)
	if err != nil {
		return err
	}
	return nil
}

func SendRequestCreatedEmail(srd *glb.ServiceDesk, email *glb.Email, request *glb.Request) error {
	if !srd.JiraInstall.Cfg.SendEmails {
		lg.Logf("don't send emails when send_emails is disabled")
		return nil
	}
	lg.Logf("sending request created email")

	templateData := struct {
		Request     *glb.Request
		ServiceDesk *glb.ServiceDesk
	}{
		Request:     request,
		ServiceDesk: srd,
	}
	subject := fmt.Sprintf("%s %s", request.IssueKey, email.Subject)
	template := srd.RequestCreationEmailTextPlainTemplate
	sendReplyEmail(srd.JiraInstall.Cfg, email, template, &templateData, subject, srd.ReplyAddress)
	return nil
}

func SendWrongAddressErrorEmail(jiraInstall *glb.JiraInstall, email *glb.Email) error {
	if !jiraInstall.Cfg.SendEmails {
		lg.Logf("don't send emails when send_emails is disabled")
		return nil
	}
	lg.Logf("sending customer attempted request creation with jira install address error email")

	templateData := struct {
		JiraInstall *glb.JiraInstall
	}{
		JiraInstall: jiraInstall,
	}
	subject := fmt.Sprintf("%s %s", jiraInstall.RejectedMailSubject, email.Subject)
	template := jiraInstall.RejectedMailTemplate
	sendReplyEmail(jiraInstall.Cfg, email, template, &templateData, subject, jiraInstall.ReplyAddress)
	return nil
}
