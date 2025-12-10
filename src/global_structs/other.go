// other structs //
package global_structs

import (
	"net/mail"
	"time"
)

type File struct {
	Name  string
	Bytes []byte
}

type Request struct {
	IssueKey      string
	ServiceDeskId string
	PortalLink    string
	Status        string
	Reporter      string
	Assignee      string
}

// everything you need to decide what to do with an incoming email
type EmailHandlingParam struct {
	Email *Email
	// true iff in Cfg.DontReplyTo
	DontReplyTo bool
	// true iff in Cfg.EmailWhitelist
	Whitelisted bool
	// the jira install this email went to, nil if invalid
	// (always defined when ServiceDesk is defined)
	JiraInstall *JiraInstall
	// when sender has jira account
	SenderJiraUsername string
	// the serviceDesk this email went to
	// (always defined when JiraInstall is defined)
	ServiceDesk *ServiceDesk
	// if a request is referenced in the email's subject
	Request *Request
	// true iff current status of request is not to comment on
	DontComment bool
	// the serviceDesk the request belongs to
	RequestServiceDesk *ServiceDesk
}

type Email struct {
	Date time.Time
	// use envelope if available, otherwise use header
	From *mail.Address
	// used for warning messages
	OrigHeaderFrom   *mail.Address
	OrigEnvelopeFrom *mail.Address
	To               []*mail.Address
	ReplyTo          *mail.Address
	Cc               []*mail.Address
	Bcc              []*mail.Address
	Subject          string
	SenderIP         string
	SpamScore        float64
	TextBody         string
	Files            []File
	IsAutoReply      bool
	IsMalware        bool
}

type NoticedOutOfOffice map[string]struct{}
