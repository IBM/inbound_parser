// turn raw email from sendgrid into parsed Email //
package email

import (
	"bytes"
	"encoding/json"
	"math"
	"mime"
	"strconv"
	"strings"
	"time"

	"net/http"
	"net/mail"

	"github.com/h2non/filetype"
	"github.com/jhillyerd/enmime/v2"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
	"github.ibmgcloud.net/dth/inbound_parser/malware_detection"
)

func isAutoReply(env *enmime.Envelope) bool {
	autoSubmitted := env.GetHeader("Auto-Submitted")
	if autoSubmitted != "" && autoSubmitted != "no" {
		return true
	}
	if env.GetHeader("X-Autoreply") != "" {
		return true
	}
	if env.GetHeader("X-Autorespond") != "" {
		return true
	}
	if env.GetHeader("Precedence") == "auto_reply" {
		return true
	}
	return false
}

func getSendgridFields(body []byte) (map[string][]string, error) {
	fakeReq, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	fakeReq.Header.Set("Content-Type", "multipart/form-data; boundary=xYzZY")
	fakeReq.Header.Set("User-Agent", "Twilio-SendGrid")

	err = fakeReq.ParseMultipartForm(1000000000)
	if err != nil {
		return nil, err
	}

	return fakeReq.MultipartForm.Value, nil
}

func getDeliveryStatusNotification(rawEmail string) (string, error) {
	fakeReq, err := http.NewRequest(http.MethodPost, "", strings.NewReader(rawEmail))
	if err != nil {
		return "", err
	}
	fakeReq.Header.Set("Content-Type", "multipart/form-data; boundary=xYzZY")
	fakeReq.Header.Set("User-Agent", "Twilio-SendGrid")

	err = fakeReq.ParseMultipartForm(1000000000)
	if err != nil {
		return "", err
	}

	for thing := range fakeReq.MultipartForm.Value {
		if strings.Contains(thing, "Content-Type: message/delivery-status") {
			return thing, nil
		}
	}
	return "", nil
}

// constructFilename interprets the mime part as an (inline) attachment and returns its filename
// If no filename is given it guesses a sensible filename for it based on the filetype.
func constructFilename(part *enmime.Part) string {
	if strings.TrimSpace(part.FileName) != "" {
		return part.FileName
	}

	filenameWOExtension := "unnamed_file"
	if strings.TrimSpace(part.ContentID) != "" {
		filenameWOExtension = part.ContentID
	}

	fileExtension := ".unknown"
	match, err := filetype.Match(part.Content)
	if err != nil {
		mimeExtensions, err := mime.ExtensionsByType(part.ContentType)
		if err == nil && len(mimeExtensions) != 0 {
			// just use the first one we find, this is just a fallback anyways
			fileExtension = mimeExtensions[0]
		}
	} else {
		// while the mime detector includes the leading dot the filetype library does not
		fileExtension = "." + match.Extension
	}
	return filenameWOExtension + fileExtension
}

func getFiles(env *enmime.Envelope) []glb.File {
	files := make([]glb.File, len(env.Inlines)+len(env.Attachments)+len(env.OtherParts))

	// get inlines
	for idx, file := range env.Inlines {
		files[idx] = glb.File{Name: constructFilename(file), Bytes: file.Content}
	}

	// get attachments
	for idx, file := range env.Attachments {
		files[len(env.Inlines)+idx] = glb.File{Name: constructFilename(file), Bytes: file.Content}
	}

	// get other parts (mostly multipart/related files, these are for example embedded images in an html mail)
	for idx, file := range env.OtherParts {
		files[len(env.Inlines)+len(env.Attachments)+idx] = glb.File{Name: constructFilename(file), Bytes: file.Content}
	}
	return files
}

func GetParsedEmail(body []byte, cfg *glb.Config) (*glb.Email, error) {
	lg.Logf("parsing email with enmime")
	// sendgrid stuff
	sendgridFields, err := getSendgridFields(body)
	if err != nil {
		lg.Logf("failed to parse sendgrid fields")
		return nil, err
	}

	rawEmail := sendgridFields["email"][0]
	subject := sendgridFields["subject"][0]
	var to []*mail.Address
	to, err = mail.ParseAddressList(sendgridFields["to"][0])
	if err != nil {
		lg.Logf("error to be ignored in sendgrid to field: %s", err.Error())
	}
	// there is a bug in sendgrid
	// sendgrid doesn't escape commas and the like with " when the address is encoded using b64
	// therefore this call will fail in such cases
	headerFrom, err := mail.ParseAddress(sendgridFields["from"][0])
	if err != nil {
		lg.Logf("error to be ignored in sendgrid from field: %s", err.Error())
		// envelope address will be used instead
		headerFrom = &mail.Address{Name: "", Address: ""}
	}
	senderIP := sendgridFields["sender_ip"][0]
	spamScore, err := strconv.ParseFloat(sendgridFields["spam_score"][0], 64)
	if err != nil {
		lg.Logf("failed to get spam score")
		return nil, err
	}

	// when the email got to the inbound_parser via bcc, the bcc address will be in this to address
	type Envelope struct {
		To   []string `json:"to"`
		From string   `json:"from"`
	}
	var envelope *Envelope
	err = json.Unmarshal([]byte(sendgridFields["envelope"][0]), &envelope)
	if err != nil {
		lg.Logf("failed to get sendgrid mail envelope")
		return nil, err
	}
	EnvelopeTo, err := mail.ParseAddress(envelope.To[0])
	if err != nil {
		lg.Logf("failed to parse to addresses")
		return nil, err
	}
	// use truest from email address -> better against phishing
	envelopeFrom, err := mail.ParseAddress(envelope.From)
	if err != nil {
		envelopeFrom = &mail.Address{Name: "", Address: ""}
	}
	from := &mail.Address{Name: headerFrom.Name, Address: headerFrom.Address}
	if from.Address == "" {
		from.Address = envelopeFrom.Address
	}
	if from.Name == "" {
		from.Name = envelopeFrom.Name
	}
	if envelopeFrom.Address != headerFrom.Address {
		lg.Logf("Warning: envelope from address: %s, header from address: %s", envelopeFrom.Address, headerFrom.Address)
	}

	env, err := enmime.ReadEnvelope(strings.NewReader(rawEmail))
	// soft parsing error, we can continue even with such an error
	// TODO: log errors to database
	for _, e := range env.Errors {
		lg.Logf("Warning: enmime decoding error: %s", e)
	}
	// hard parsing error
	if err != nil {
		lg.Logf("failed to get enmime envelope")
		return nil, err
	}

	emailBody := strings.TrimSpace(env.Text[:int(math.Min(float64(len(env.Text)), 32767))])
	files := getFiles(env)

	cc, err := env.AddressList("Cc")
	if err != nil {
		if err == mail.ErrHeaderNotPresent {
			cc = make([]*mail.Address, 0)
		} else {
			lg.Logf("failed to parse cc addresses")
			return nil, err
		}
	}

	bcc, err := env.AddressList("Bcc")
	if err != nil {
		if err == mail.ErrHeaderNotPresent {
			bcc = make([]*mail.Address, 0)
		} else {
			lg.Logf("failed to parse bcc addresses")
			return nil, err
		}
	}

	allReplyTo, err := env.AddressList("Reply-To")
	if err != nil {
		if err == mail.ErrHeaderNotPresent {
			allReplyTo = make([]*mail.Address, 0)
		} else {
			lg.Logf("failed to parse reply-to addresses")
			return nil, err
		}
	}
	var replyTo *mail.Address
	replyTo = nil
	// when there are more than one 'reply to' addresses, ignore all except for the first
	if len(allReplyTo) != 0 {
		replyTo = allReplyTo[0]
	}

	isMalware, err := malware_detection.ContainsMalware(files, cfg)
	if err != nil {
		lg.Logf("failed to detect malware")
		return nil, err
	}

	// TODO: test if this is needed
	// e.g. for bounce mails
	// if strings.Split(actualEmail.ContentType, ";")[0] == "multipart/report" {
	//     emailBody = rawEmail
	// }

	date, err := env.Date()
	// TODO: log errors to database
	if err != nil {
		lg.Logf("Warning: failed to parse email date: %s", err)
		// TODO: use time the email arrived at the inbound_parser
		date = time.Now()
	}

	mail := glb.Email{
		Date:             date,
		From:             from,
		OrigHeaderFrom:   headerFrom,
		OrigEnvelopeFrom: envelopeFrom,
		To:               append(to, EnvelopeTo),
		ReplyTo:          replyTo,
		Cc:               cc,
		Bcc:              bcc,
		Subject:          subject,
		SenderIP:         senderIP,
		SpamScore:        spamScore,
		TextBody:         emailBody,
		Files:            files,
		IsAutoReply:      isAutoReply(env),
		IsMalware:        isMalware,
	}
	return &mail, nil
}
