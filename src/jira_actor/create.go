// low level jira interaction to put info into jira //
package jira_actor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"regexp"
	"strings"
	"unicode"

	jira "github.com/andygrunwald/go-jira"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
)

var forbiddenFileNameChars = regexp.MustCompile(`(\\")|[\\/%:$?*]`)

func cleanUnicode(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, string(str))
}

// length is limited by Jira
func capLength(str string, length int, emptyAllowed bool) string {
	str = cleanUnicode(str)
	str = strings.TrimSpace(str)
	if str == "" && !emptyAllowed {
		str = "<blank>"
	}
	return str[:int(math.Min(float64(len(str)), float64(length)))]
}

// return id of request
func CreateRequest(summary string, description string, reporterUsername string, requestTypeId string, serviceDeskId string, tryAnonymous bool, client *jira.Client) (string, error) {
	summary = capLength(summary, 255, false)
	description = capLength(description, 32767, false)
	lg.Logf("creating request with summary '%s' from '%s'\n", summary, reporterUsername)
	newRequest := &jira.Request{
		ServiceDeskID: serviceDeskId,
		TypeID:        requestTypeId,
		FieldValues: []jira.RequestFieldValue{
			{
				FieldID: "summary",
				Value:   summary,
			},
			{
				FieldID: "description",
				Value:   description,
			},
		},
	}
	request, resp, err := client.Request.Create(reporterUsername, []string{}, newRequest)
	if err != nil {
		lg.Logf("failed to create request on behalf of '%s'\n", reporterUsername)
		if reporterUsername != "" && tryAnonymous {
			lg.Logf("trying again anonymously")
			return CreateRequest(summary, description, "", requestTypeId, serviceDeskId, false, client)
		}
		printJiraResponse(resp)
		return "", err
	}
	return request.IssueKey, nil
}

func CreateComment(commentBody string, files []glb.File, IssueKey string, serviceDeskId string, client *jira.Client) error {
	lg.Logf("creating comment\n")
	var tempFiles []string
	for _, file := range files {
		tempFile, err := createTempFile(file, serviceDeskId, client)
		if err != nil {
			return err
		}
		tempFiles = append(tempFiles, tempFile)
	}
	err := createCommentFromTempFiles(commentBody, tempFiles, IssueKey, client)
	if err != nil {
		return err
	}
	return nil
}

// return attachment id
func createTempFile(file glb.File, serviceDeskId string, client *jira.Client) (string, error) {
	lg.Logf("creating temp file %s\n", file.Name)
	endpoint := fmt.Sprintf("/rest/servicedeskapi/servicedesk/%s/attachTemporaryFile", serviceDeskId)
	b := new(bytes.Buffer)
	writer := multipart.NewWriter(b)

	fileName := capLength(file.Name, 50, false)
	fileName = forbiddenFileNameChars.ReplaceAllString(fileName, "")
	fw, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}

	_, err = fw.Write(file.Bytes)
	if err != nil {
		return "", err
	}
	writer.Close()

	req, err := client.NewMultiPartRequestWithContext(context.Background(), "POST", endpoint, b)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-ExperimentalApi", "opt-in")
	req.Header.Set("X-Atlassian-Token", "no-check")

	type TemporaryFile struct {
		TemporaryAttachmentId string `json:"temporaryAttachmentId"`
		FileName              string `json:"fileName"`
	}
	type TemporaryFiles struct {
		TemporaryAttachments []TemporaryFile `json:"temporaryAttachments"`
	}

	var tempFiles TemporaryFiles
	resp, err := client.Do(req, &tempFiles)
	if err != nil {
		printJiraResponse(resp)
		return "", err
	}
	if len(tempFiles.TemporaryAttachments) != 1 {
		return "", errors.New("temporary file attachment failed")
	}

	return tempFiles.TemporaryAttachments[0].TemporaryAttachmentId, nil
}

func createCommentFromTempFiles(commentBody string, tempFiles []string, issueKey string, client *jira.Client) error {
	commentBody = capLength(commentBody, 32767, true)
	lg.Logf("creating comment from temp files\n")
	endpoint := fmt.Sprintf("/rest/servicedeskapi/request/%s/attachment", issueKey)
	type AdditionalComment struct {
		Body string `json:"body"`
	}
	type AttachmentComment struct {
		TemporaryAttachmentIds []string          `json:"temporaryAttachmentIds"`
		Public                 bool              `json:"public"`
		AdditionalComment      AdditionalComment `json:"additionalComment"`
	}
	data := AttachmentComment{
		TemporaryAttachmentIds: tempFiles,
		Public:                 true,
		AdditionalComment: AdditionalComment{
			Body: commentBody,
		},
	}
	req, err := client.NewRequestWithContext(context.Background(), "POST", endpoint, data)
	if err != nil {
		return err
	}
	req.Header.Set("X-ExperimentalApi", "opt-in")

	resp, err := client.Do(req, nil)
	if err != nil {
		printJiraResponse(resp)
		return err
	}
	return nil
}

func AddParticipant(issueKey string, username string, client *jira.Client) error {
	lg.Logf("adding participant %s to %s\n", username, issueKey)
	apiEndpoint := fmt.Sprintf("/rest/servicedeskapi/request/%s/participant", issueKey)
	type AddParticipantRequest struct {
		Usernames []string `json:"usernames"`
	}
	body := AddParticipantRequest{
		Usernames: []string{username},
	}
	req, err := client.NewRequestWithContext(context.Background(), "POST", apiEndpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req, nil)
	if err != nil {
		printJiraResponse(resp)
		return err
	}
	return nil
}

func CreateCustomer(email string, fullName string, adminClient *jira.Client) error {
	if fullName == "" {
		fullName = email
	}
	// , not allowed
	fullName = strings.ReplaceAll(fullName, ",", "")
	fullName = capLength(fullName, 60, false)
	lg.Logf("creating customer '%s' '%s'\n", email, fullName)
	endpoint := fmt.Sprintf("/rest/servicedeskapi/customer")
	type CustomerCreation struct {
		Email    string `json:"email"`
		FullName string `json:"fullName"`
	}
	data := CustomerCreation{
		Email:    email,
		FullName: fullName,
	}
	req, err := adminClient.NewRequestWithContext(context.Background(), "POST", endpoint, data)
	if err != nil {
		return err
	}
	req.Header.Set("X-ExperimentalApi", "opt-in")

	resp, err := adminClient.Do(req, nil)
	if err != nil {
		printJiraResponse(resp)
		return err
	}
	return nil
}
