// low level jira interaction to get info out of jira //
package jira_actor

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	jira "github.com/andygrunwald/go-jira"

	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
	lg "github.ibmgcloud.net/dth/inbound_parser/logging"
)

func GetJiraClient(url string, token string) (*jira.Client, error) {
	lg.Logf("getting jira client for %s\n", url)
	tp := jira.BearerAuthTransport{
		Token: token,
	}
	return jira.NewClient(tp.Client(), url)
}

func GetJiraUsername(emailAddress string, emailName string, client *jira.Client) (string, error) {
	lg.Logf("getting user for %s %s\n", emailAddress, emailName)
	users, resp, err := client.User.Find("", jira.WithUsername(url.QueryEscape(emailAddress)))
	if err != nil {
		printJiraResponse(resp)
		return "", err
	}

	// Jira only performs fuzzy search -> figure out what user to use
	var usersWithCorrectEmail []jira.User
	for _, user := range users {
		if user.EmailAddress == emailAddress {
			usersWithCorrectEmail = append(usersWithCorrectEmail, user)
			if user.DisplayName == emailName {
				// found with exact match
				return user.Name, nil
			}
		}
	}
	if len(usersWithCorrectEmail) != 0 {
		// found without name match
		return usersWithCorrectEmail[0].Name, nil
	}
	// not found
	lg.Logf("user not found")
	return "", nil
}

// don't return an error when no request was found -> return nil request instead
func GetRequest(issueKey string, client *jira.Client) (*glb.Request, error) {
	issue, resp, err := client.Issue.Get(issueKey, nil)
	if err != nil {
		printJiraResponse(resp)
		return nil, nil
	}
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.Name
	}

	lg.Logf("getting request %s\n", issueKey)
	endpoint := fmt.Sprintf("/rest/servicedeskapi/request/%s", issueKey)
	req, err := client.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	type ReturnedRequestLinks struct {
		PortalLink string `json:"web"`
	}
	type CurrentStatus struct {
		Status string `json:"status"`
	}
	type Reporter struct {
		Name string `json:"name"`
	}
	type ReturnedRequest struct {
		IssueKey      string               `json:"issueKey"`
		ServiceDeskId string               `json:"serviceDeskId"`
		Links         ReturnedRequestLinks `json:"_links"`
		CurrentStatus CurrentStatus        `json:"currentStatus"`
		Reporter      Reporter             `json:"reporter"`
	}
	var returnedRequest ReturnedRequest
	resp, err = client.Do(req, &returnedRequest)
	if err != nil {
		printJiraResponse(resp)
		return nil, nil
	}
	return &glb.Request{
		IssueKey:      returnedRequest.IssueKey,
		ServiceDeskId: returnedRequest.ServiceDeskId,
		PortalLink:    returnedRequest.Links.PortalLink,
		Status:        returnedRequest.CurrentStatus.Status,
		Reporter:      returnedRequest.Reporter.Name,
		Assignee:      assignee,
	}, nil
}

func GetRequestTypeId(typeName string, serviceDeskId string, client *jira.Client) (string, error) {
	lg.Logf("getting request type for %s\n", typeName)
	apiEndpoint := fmt.Sprintf("/rest/servicedeskapi/servicedesk/%s/requesttype", serviceDeskId)
	req, err := client.NewRequestWithContext(context.Background(), "GET", apiEndpoint, nil)
	if err != nil {
		return "", err
	}
	type RequestTypeResponse struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	type RequestTypesResponse struct {
		IsLastPage bool                  `json:"isLastPage"`
		Values     []RequestTypeResponse `json:"values"`
	}
	var requestTypesResponse RequestTypesResponse
	resp, err := client.Do(req, &requestTypesResponse)
	if err != nil {
		printJiraResponse(resp)
		return "", err
	}
	if !requestTypesResponse.IsLastPage {
		return "", errors.New("There are more request types than fit on one page. Pagination parsing isn't implemented.")
	}
	for _, requestTypeResponse := range requestTypesResponse.Values {
		if requestTypeResponse.Name == typeName {
			lg.Logf("found type id: %s\n", requestTypeResponse.Id)
			return requestTypeResponse.Id, nil
		}
	}
	return "", errors.New(fmt.Sprintf("Didn't find request type '%s' in serviceDeskId '%s', check: %s for the proper request type name.",
		typeName, serviceDeskId, apiEndpoint))
}

// TODO: cache for multiple projects, don't run same request multiple times
func GetServiceDeskId(projectKey string, client *jira.Client) (string, error) {
	lg.Logf("getting serviceDesk id for project %s\n", projectKey)
	perPage := 1
	start := 0
	limit := 0
	for true {
		start = limit
		limit = start + perPage

		apiEndpoint := fmt.Sprintf("/rest/servicedeskapi/servicedesk")
		req, err := client.NewRequestWithContext(context.Background(), "GET", apiEndpoint, nil)
		if err != nil {
			return "", err
		}
		req.URL.Query().Add("start", strconv.Itoa(start))
		req.URL.Query().Add("limit", strconv.Itoa(limit))

		type ServiceDeskResponse struct {
			Id         string `json:"id"`
			ProjectKey string `json:"projectKey"`
		}
		type ServiceDesksResponse struct {
			Start      int                   `json:"start"`
			Limit      int                   `json:"limit"`
			Size       int                   `json:"size"`
			IsLastPage bool                  `json:"isLastPage"`
			Values     []ServiceDeskResponse `json:"values"`
		}
		var serviceDesksResponse ServiceDesksResponse
		resp, err := client.Do(req, &serviceDesksResponse)
		if err != nil {
			printJiraResponse(resp)
			return "", err
		}
		for _, serviceDeskResponse := range serviceDesksResponse.Values {
			if serviceDeskResponse.ProjectKey == projectKey {
				lg.Logf("found id: %s\n", serviceDeskResponse.Id)
				return serviceDeskResponse.Id, nil
			}
		}
		// serviceDesksResponse.Limit is 0 when the serviceDesk api doesn't return valid json or when there are no serviceDesks
		if serviceDesksResponse.IsLastPage || serviceDesksResponse.Limit == 0 {
			break
		}
		lg.Logf("didn't get to last serviceDeskId page")
	}
	return "", errors.New(fmt.Sprintf("couldn't find project with key '%s'", projectKey))
}
