// email helper structs/functions //
package email

import (
	"fmt"
	"net/mail"
	"strings"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
)

func FormatAddr(address *mail.Address) string {
	return fmt.Sprintf("%s <%s>", address.Name, address.Address)
}

func getQuotedTextBody(email *glb.Email) string {
	out := email.Date.Format("2 January 2006 15:04:05 ") + email.From.Address + "\n\n"
	for _, line := range strings.Split(email.TextBody, "\n") {
		out += "> " + line + "\n"
	}
	return out
}

func GetEmailStatsStr(email *glb.Email) string {
	// time zone used in the email
	tz, _ := email.Date.Local().Zone()
	outStr := fmt.Sprintf("At: %s (%s)\n", email.Date.Format("02.01.2006 15:04:05"), tz)

	outStr += "To: "
	for _, toAddress := range email.To {
		outStr += FormatAddr(toAddress) + ", "
	}
	outStr = strings.TrimSuffix(outStr, ", ")
	outStr += "\n"

	outStr += fmt.Sprintf("From: %s\n", FormatAddr(email.From))
	if email.ReplyTo != nil {
		outStr += fmt.Sprintf("Reply To: %s\n", FormatAddr(email.ReplyTo))
	}

	if len(email.Cc) > 0 {
		outStr += "Cc: "
		for _, ccAddress := range email.Cc {
			outStr += FormatAddr(ccAddress) + ", "
		}
		outStr = strings.TrimSuffix(outStr, ", ")
		outStr += "\n"
	}

	if len(email.Bcc) > 0 {
		outStr += "Bcc: "
		for _, bccAddress := range email.Bcc {
			outStr += FormatAddr(bccAddress) + ", "
		}
		outStr = strings.TrimSuffix(outStr, ", ")
		outStr += "\n"
	}

	outStr += fmt.Sprintf("Subject: %s\n", email.Subject)
	outStr += fmt.Sprintf("Spam Score: %f\n", email.SpamScore)
	outStr += fmt.Sprintf("Envelope From: %s\n", FormatAddr(email.OrigEnvelopeFrom))
	outStr += fmt.Sprintf("Header From: %s\n", FormatAddr(email.OrigHeaderFrom))
	return outStr
}
