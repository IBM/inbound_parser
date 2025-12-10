// single source of truth for config, unchanged except for config loading //
package global_structs

import (
	"html/template"
	"net/mail"

	jira "github.com/andygrunwald/go-jira"
)

type ServiceDesk struct {
	// defined later on
	JiraInstall *JiraInstall

	ProjectKey string `yaml:"project_key"`
	// defined later
	Id string

	CreateEventRequests bool `yaml:"create_event_requests"`
	// defined later
	OnlyCreateEventRequests bool
	Emails                  []string `yaml:"emails"`
	ReplyEmailName          string   `yaml:"reply_email_name"`
	// defined later
	ReplyAddress *mail.Address

	RequestType string `yaml:"request_type"`
	// defined later
	RequestTypeId  string
	RequestPostfix string `yaml:"request_postfix"`
	// only when SendEmails
	RequestCreationEmailTextPlainPath string `yaml:"request_creation_email_text_plain_path"`
	// defined later on
	RequestCreationEmailTextPlainTemplate *template.Template

	DontCommentRequestStatus []string `yaml:"dont_comment_request_status"`

	ReplyAboveThis string `yaml:"reply_above_this"`
}

type JiraInstall struct {
	// defined later on
	Cfg         *Config
	Client      *jira.Client
	AdminClient *jira.Client

	URL        string `yaml:"url"`
	Token      string `yaml:"token"`
	AdminToken string `yaml:"admin_token"`

	Emails []string `yaml:"emails"`
	// only when emails are defined
	// may be left blank
	RejectedMailSubject string `yaml:"rejected_mail_subject"`
	RejectedMailPath    string `yaml:"rejected_mail_template_path"`
	// optional
	ReplyEmailName string `yaml:"reply_email_name"`
	// defined later on
	RejectedMailTemplate *template.Template
	ReplyAddress         *mail.Address

	ServiceDesks []*ServiceDesk `yaml:"servicedesks"`
}

type Config struct {
	CriticalMailTo   string `yaml:"critical_mail_to"`
	CriticalMailFrom string `yaml:"critical_mail_from"`

	DumpRequests         bool   `yaml:"dump_requests"`
	ParseRequests        bool   `yaml:"parse_requests"`
	DebugParseOnly       bool   `yaml:"debug_parse_only"`
	SendEmails           bool   `yaml:"send_emails"`
	HandleEvents         bool   `yaml:"handle_events"`
	HandleEventsUsername string `yaml:"handle_events_username"`
	// defined later on
	HandleEventsSrd *ServiceDesk
	PrintLicenses   bool `yaml:"print_licenses"`

	CheckMalware  bool   `yaml:"check_malware"`
	ClamAVScandir string `yaml:"clamav_scandir"`

	DumpDir       string `yaml:"dump_dir"`
	EmailKeepDays int    `yaml:"email_keep_days"`
	// needs to be bigger than 0
	MaxSpamScore float64 `yaml:"max_spam_score"`

	// only when DumpRequests
	Port          int    `yaml:"port"`
	Domain        string `yaml:"domain"`
	SSLCert       string `yaml:"ssl_cert"`
	SSLKey        string `yaml:"ssl_key"`
	SendgridToken string `yaml:"sendgrid_token"`

	// only when ParseRequests
	JiraInstalls    []*JiraInstall `yaml:"jira_installs"`
	EmailWhitelist  []string       `yaml:"email_whitelist"`
	MaxParticipants uint           `yaml:"max_participants"`

	// only when SendEmails
	SendEMailHost     string   `yaml:"send_email_host"`
	SendEMailPort     int      `yaml:"send_email_port"`
	DontReplyToEmails []string `yaml:"dont_reply_to_emails"`
}
