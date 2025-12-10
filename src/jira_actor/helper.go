// low level jira interaction //
package jira_actor

import (
	"bytes"
	"io"

	jira "github.com/andygrunwald/go-jira"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

// used for debugging errors
func printJiraResponse(resp *jira.Response) {
	if resp == nil {
		return
	}
	body := bytes.NewBuffer(nil)
	resp.Request.Write(body)
	resp.Response.Write(body)
	bodyBytes, _ := io.ReadAll(body)
	lg.Logf(string(bodyBytes))
}
