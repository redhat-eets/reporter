package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	jiraIssuesEndpoint = "/rest/api/2/issue/"
)

// JiraClient manages communication with the Jira REST API.
type JiraClient struct {
	ServerURL   string
	AccessToken string
}

// JiraIssue represents an Issue as returned by the Jira REST API.
type JiraIssue struct {
	ID          string
	Type        string
	Parent      *JiraIssue
	Summary     string
	Description string
	Labels      []string
	SubTasks    []*JiraIssue
}

// IsLabeledWithAnyOf provides a convenience method to check whether the Jira Issue
// is labeled with at least one of the labels given by the user.
func (issue *JiraIssue) IsLabeledWithAnyOf(ls []string) bool {
	for _, label := range ls {
		if slices.Contains(issue.Labels, label) {
			return true
		}
	}

	return false
}

type httpResponse struct {
	Body       []byte
	StatusCode int
}

type apiResponseIssue struct {
	Key    string                 `json:"key"`
	Fields apiResponseIssueFields `json:"fields"`
}

type apiResponseIssueType struct {
	Name      string `json:"name"`
	IsSubTask bool   `json:"subtask"`
}

type apiResponseIssueFields struct {
	IssueType   apiResponseIssueType `json:"issuetype"`
	Parent      *apiResponseIssue    `json:"parent"`
	Labels      []string             `json:"labels"`
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	SubTasks    []*apiResponseIssue  `json:"subtasks"`
}

// JiraIssue converts an Issue returned by the Jira REST API to the internal representation.
func (r httpResponse) JiraIssue() (issue JiraIssue, err error) {
	var data apiResponseIssue
	if err = json.Unmarshal(r.Body, &data); err != nil {
		return issue, err
	}

	issue, err = data.JiraIssue()
	if err != nil {
		return issue, err
	}

	return issue, nil
}

// JiraIssue converts an Issue returned by the Jira REST API to the internal representation.
func (b *apiResponseIssue) JiraIssue() (JiraIssue, error) {
	parent := JiraIssue{}
	if b.Fields.Parent != nil {
		parent, err := b.Fields.Parent.JiraIssue()
		if err != nil {
			return parent, errors.New("parent issue could not be converted to JiraIssue")
		}
	}

	var subtasks []*JiraIssue
	for i := range b.Fields.SubTasks {
		subtask, err := b.Fields.SubTasks[i].JiraIssue()
		if err != nil {
			return subtask, errors.New("sub-task could not be converted to JiraIssue")
		}
		subtasks = append(subtasks, &subtask)
	}

	issue := JiraIssue{
		ID:          b.Key,
		Type:        b.Fields.IssueType.Name,
		Parent:      &parent,
		Summary:     b.Fields.Summary,
		Description: b.Fields.Description,
		Labels:      b.Fields.Labels,
		SubTasks:    subtasks,
	}

	return issue, nil
}

func (c JiraClient) prepareRequest(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header = http.Header{
		"Authorization": {fmt.Sprintf("Bearer %s", c.AccessToken)},
		"Content-Type":  {"application/json"},
	}

	return req, nil
}

func (c JiraClient) sendRequest(method string, url string, body io.Reader) (resp httpResponse, err error) {
	client := http.Client{}
	httpReq, err := c.prepareRequest(method, url, body)
	if err != nil {
		return resp, err
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return resp, err
	}

	defer httpResp.Body.Close()
	b, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, err
	}

	resp.Body = b
	resp.StatusCode = httpResp.StatusCode

	return resp, nil
}

// GetIssue sends a request to Jira REST API to fetch an Issue with the given ID.
func (c JiraClient) GetIssue(id string) (issue JiraIssue, err error) {
	InfoLog.Printf("Getting Jira issue '%s'", id)

	endpointURL, err := url.JoinPath(c.ServerURL, jiraIssuesEndpoint, id)
	if err != nil {
		return issue, err
	}

	resp, err := c.sendRequest("GET", endpointURL, nil)
	if err != nil {
		return issue, err
	}

	if resp.StatusCode != http.StatusOK {
		return issue, fmt.Errorf("issue %s could not be fetched. HTTP status code: %d", id, resp.StatusCode)
	}

	issue, err = resp.JiraIssue()
	if err != nil {
		return issue, err
	}

	return issue, nil
}

type apiRequestIssueUpdateFields struct {
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
}

// WrapAndMarshalJSON returns a JSON-encoded payload wrapped in a structure
// required by the Jira REST API.
func (f apiRequestIssueUpdateFields) WrapAndMarshalJSON() ([]byte, error) {
	body := map[string]any{
		"update": map[string]any{
			"summary": []map[string]string{
				{"set": f.Summary},
			},
			"description": []map[string]string{
				{"set": f.Description},
			},
			"labels": []map[string][]string{
				{"set": f.Labels},
			},
		},
	}

	res, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// UpdateIssue sends a request to Jira REST API to update an Issue with the given ID.
func (c JiraClient) UpdateIssue(id string, summary string, description string, labels []string) error {
	InfoLog.Printf("Updating Jira issue '%s' (%s, %v)", id, summary, labels)

	endpointURL, err := url.JoinPath(c.ServerURL, jiraIssuesEndpoint, id)
	if err != nil {
		return err
	}

	fields := apiRequestIssueUpdateFields{
		Summary:     summary,
		Description: description,
		Labels:      labels,
	}

	payload, err := fields.WrapAndMarshalJSON()
	if err != nil {
		return err
	}

	resp, err := c.sendRequest("PUT", endpointURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("issue %s could not be updated. Reason: %w", id, err)
	}

	// Jira returns HTTP Status Code 204 if the update operation was successful
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("issue %s could not be updated. HTTP status code: %d", id, resp.StatusCode)
	}

	return nil
}

type apiRequestIssueCreateFields struct {
	Project     map[string]string `json:"project"`
	Parent      map[string]string `json:"parent"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Labels      []string          `json:"labels"`
	IssueType   map[string]string `json:"issuetype"`
}

// WrapAndMarshalJSON returns a JSON-encoded payload in a structure
// required by the Jira REST API.
func (f apiRequestIssueCreateFields) WrapAndMarshalJSON() ([]byte, error) {
	body := map[string]apiRequestIssueCreateFields{
		"fields": f,
	}

	res, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type apiResponseIssueCreate struct {
	Key  string `json:"key"`
	Self string `json:"self"`
}

func getProjectFromParentIssueID(id string) string {
	// It is possible that this is too simple and will not work on all Jira projects
	elem := strings.Split(id, "-")
	return strings.Join(elem[:len(elem)-1], "")
}

// CreateSubtask sends a request to create a Sub-task under a given parent Issue.
func (c JiraClient) CreateSubtask(parent string, summary string, description string, labels []string) (string, error) {
	InfoLog.Printf("Creating a new Jira issue under '%s'", parent)

	endpointURL, err := url.JoinPath(c.ServerURL, jiraIssuesEndpoint)
	if err != nil {
		return "", err
	}

	project := getProjectFromParentIssueID(parent)
	fields := apiRequestIssueCreateFields{
		Project:     map[string]string{"key": project},
		Parent:      map[string]string{"key": parent},
		Summary:     summary,
		Description: description,
		Labels:      labels,
		IssueType:   map[string]string{"name": "Sub-task"},
	}

	payload, err := fields.WrapAndMarshalJSON()
	if err != nil {
		return "", err
	}

	resp, err := c.sendRequest("POST", endpointURL, bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("sub-task could not be created. Reason: %w", err)
	}

	// Jira REST API returns Status Code 201 if issue was created
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("sub-task could not be created. HTTP status code: %d", resp.StatusCode)
	}

	var data apiResponseIssueCreate
	if err = json.Unmarshal(resp.Body, &data); err != nil {
		return "", err
	}

	return data.Key, nil
}
