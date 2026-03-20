package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const BaseURL = "https://api.clickup.com/api/v2"

type Client struct {
	Token      string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) doReq(method, endpoint string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

// Teams --------------------------

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) GetTeams() ([]Team, error) {
	data, err := c.doReq("GET", "/team", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Teams []Team `json:"teams"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Teams, nil
}

// Spaces -------------------------

type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) GetSpaces(teamID string) ([]Space, error) {
	endpoint := fmt.Sprintf("/team/%s/space", teamID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Spaces []Space `json:"spaces"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Spaces, nil
}

// Lists --------------------------

type Folder struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Lists []List `json:"lists"`
}

type List struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) GetSpaceLists(spaceID string) ([]List, error) {
	// First fetch all folders and extract their lists
	folderEndpoint := fmt.Sprintf("/space/%s/folder", spaceID)
	folderData, err := c.doReq("GET", folderEndpoint, nil)
	if err != nil {
		return nil, err
	}

	var fResult struct {
		Folders []Folder `json:"folders"`
	}
	_ = json.Unmarshal(folderData, &fResult) // Ignore error, check lists logic later

	var allLists []List
	for _, f := range fResult.Folders {
		allLists = append(allLists, f.Lists...)
	}

	// Then fetch folderless lists
	listEndpoint := fmt.Sprintf("/space/%s/list", spaceID)
	listData, err := c.doReq("GET", listEndpoint, nil)
	if err != nil {
		return nil, err
	}

	var lResult struct {
		Lists []List `json:"lists"`
	}
	if err := json.Unmarshal(listData, &lResult); err != nil {
		return nil, err
	}

	allLists = append(allLists, lResult.Lists...)
	return allLists, nil
}

// Tasks --------------------------

type TaskStatus struct {
	Status string `json:"status"`
	Color  string `json:"color"`
}

type Assignee struct {
	Username string `json:"username"`
}

type Task struct {
	ID          string     `json:"id"`
	CustomID    string     `json:"custom_id"`
	Name        string     `json:"name"`
	Desc        string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Assignees   []Assignee `json:"assignees"`
	URL         string     `json:"url"`
	Points      *float64   `json:"points"`
}

func (c *Client) GetTasks(listID string) ([]Task, error) {
	endpoint := fmt.Sprintf("/list/%s/task", listID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Tasks, nil
}

// AddComment is a mock for now (requires view task detail parsing or specific endpoint)
func (c *Client) AddComment(taskID, comment string) error {
	// Real ClickUp Create Comment endpoint is POST /task/{task_id}/comment
	endpoint := fmt.Sprintf("/task/%s/comment", taskID)
	
	reqBody := map[string]interface{}{
		"comment_text": comment,
		"notify_all":   true,
	}
	body, _ := json.Marshal(reqBody)
	
	_, err := c.doReq("POST", endpoint, body)
	return err
}

// UpdateStatus 
func (c *Client) UpdateStatus(taskID, status string) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	
	reqBody := map[string]interface{}{
		"status": status,
	}
	body, _ := json.Marshal(reqBody)
	
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

// UpdatePoints updates the sprint points of a task
func (c *Client) UpdatePoints(taskID string, points float64) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	
	reqBody := map[string]interface{}{
		"points": points,
	}
	body, _ := json.Marshal(reqBody)
	
	_, err := c.doReq("PUT", endpoint, body)
	return err
}
