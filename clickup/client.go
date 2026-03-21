package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const BaseURL = "https://api.clickup.com/api/v2"

type Client struct {
	Token      string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token:      token,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
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

type SpaceHierarchy struct {
	Folders []Folder `json:"folders"`
	Lists   []List   `json:"lists"`
}

func (c *Client) GetSpaceLists(spaceID string) (*SpaceHierarchy, error) {
	// First fetch all folders and extract their lists
	folderEndpoint := fmt.Sprintf("/space/%s/folder", spaceID)
	folderData, err := c.doReq("GET", folderEndpoint, nil)
	if err != nil {
		return nil, err
	}

	var fResult struct {
		Folders []Folder `json:"folders"`
	}
	if err := json.Unmarshal(folderData, &fResult); err != nil {
		return nil, err
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

	return &SpaceHierarchy{
		Folders: fResult.Folders,
		Lists:   lResult.Lists,
	}, nil
}

func (c *Client) CreateSpace(teamID, name string) (*Space, error) {
	endpoint := fmt.Sprintf("/team/%s/space", teamID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("POST", endpoint, body)
	if err != nil {
		return nil, err
	}

	var space Space
	if err := json.Unmarshal(data, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

func (c *Client) CreateList(folderID, name string) (*List, error) {
	endpoint := fmt.Sprintf("/folder/%s/list", folderID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("POST", endpoint, body)
	if err != nil {
		return nil, err
	}

	var list List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *Client) CreateFolderlessList(spaceID, name string) (*List, error) {
	endpoint := fmt.Sprintf("/space/%s/list", spaceID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("POST", endpoint, body)
	if err != nil {
		return nil, err
	}

	var list List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Tasks --------------------------

type TaskStatus struct {
	Status string `json:"status"`
	Color  string `json:"color"`
}

type Assignee struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type Priority struct {
	ID         string `json:"id"`
	Priority   string `json:"priority"`
	Color      string `json:"color"`
	OrderIndex string `json:"orderindex"`
}

type Member struct {
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

type Comment struct {
	ID          string    `json:"id"`
	CommentText string    `json:"comment_text"`
	User        Assignee  `json:"user"`
	Date        string    `json:"date"`
	Parent      *string   `json:"parent,omitempty"`
	Replies     []Comment `json:"replies"`
	ReplyCount  int       `json:"reply_count"`
}

type Attachment struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Extension  string `json:"extension"`
	URL        string `json:"url"`
	URLWithQuery string `json:"url_w_query"`
}

func (c *Client) GetTeamMembers(teamID string) ([]Member, error) {
	endpoint := fmt.Sprintf("/team/%s/member", teamID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil { return nil, err }
	var result struct {
		Members []Member `json:"members"`
	}
	if err := json.Unmarshal(data, &result); err != nil { return nil, err }
	return result.Members, nil
}

type Task struct {
	ID                  string       `json:"id"`
	CustomID            string       `json:"custom_id"`
	Name                string       `json:"name"`
	Desc                string       `json:"description"`
	Status              TaskStatus   `json:"status"`
	Assignees           []Assignee   `json:"assignees"`
	URL                 string       `json:"url"`
	Points              *float64     `json:"points"`
	Priority            *Priority    `json:"priority,omitempty"`
	Parent              *string      `json:"parent,omitempty"`
	Attachments         []Attachment `json:"attachments"`
	DateCreated         string       `json:"date_created"`
	Creator             Assignee     `json:"creator"`
	MarkdownDescription string `json:"markdown_description"`
}

var escapedMarkdownCharRE = regexp.MustCompile(`\\+([*_` + "`" + `~\[\]\(\)#>!\-\+\|])`)

func unescapeMarkdownText(s string) string {
	if s == "" || !strings.Contains(s, `\`) {
		return s
	}
	for {
		next := escapedMarkdownCharRE.ReplaceAllString(s, "$1")
		if next == s {
			return s
		}
		s = next
	}
}

func normalizeTask(task *Task) {
	task.Desc = unescapeMarkdownText(task.Desc)
	task.MarkdownDescription = unescapeMarkdownText(task.MarkdownDescription)
	if task.MarkdownDescription != "" {
		task.Desc = task.MarkdownDescription
	}
}

func (c *Client) GetTasks(listID string) ([]Task, error) {
	endpoint := fmt.Sprintf("/list/%s/task?subtasks=true&include_markdown_description=true", listID)
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
	for i := range result.Tasks {
		normalizeTask(&result.Tasks[i])
	}
	return result.Tasks, nil
}

// GetTask fetches a single task by its ClickUp ID or Custom ID
func (c *Client) GetTask(taskID string, teamID string) (*Task, error) {
	if strings.Contains(taskID, "-") {
		taskID = strings.ToUpper(taskID)
	}
	endpoint := fmt.Sprintf("/task/%s?include_markdown_description=true", taskID)
	if teamID != "" && strings.Contains(taskID, "-") {
		endpoint = fmt.Sprintf("/task/%s?custom_task_ids=true&team_id=%s&include_markdown_description=true", taskID, teamID)
	}
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	normalizeTask(&task)
	return &task, nil
}

func (c *Client) GetTeamTasks(teamID string, page int) ([]Task, error) {
	endpoint := fmt.Sprintf("/team/%s/task?page=%d&subtasks=true&include_closed=true&include_markdown_description=true", teamID, page)
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
	for i := range result.Tasks {
		normalizeTask(&result.Tasks[i])
	}
	return result.Tasks, nil
}

// AddComment is a mock for now (requires view task detail parsing or specific endpoint)
func (c *Client) AddComment(taskID, comment, parentID string) error {
	var endpoint string
	if parentID != "" {
		endpoint = fmt.Sprintf("/comment/%s/reply", parentID)
	} else {
		endpoint = fmt.Sprintf("/task/%s/comment", taskID)
	}
	
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

// CreateTask creates a new task in a given list
func (c *Client) CreateTask(listID, name string, assignees []int) (*Task, error) {
	endpoint := fmt.Sprintf("/list/%s/task", listID)
	reqBody := map[string]interface{}{"name": name}
	if len(assignees) > 0 {
		reqBody["assignees"] = assignees
	}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("POST", endpoint, body)
	if err != nil {
		return nil, err
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	normalizeTask(&task)
	return &task, nil
}

// CreateSubtask creates a new subtask under a parent task
func (c *Client) CreateSubtask(listID, parentID, name string, assignees []int) (*Task, error) {
	endpoint := fmt.Sprintf("/list/%s/task", listID)
	reqBody := map[string]interface{}{"name": name, "parent": parentID}
	if len(assignees) > 0 {
		reqBody["assignees"] = assignees
	}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("POST", endpoint, body)
	if err != nil {
		return nil, err
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	normalizeTask(&task)
	return &task, nil
}

// DeleteTask deletes a task by ID
func (c *Client) DeleteTask(taskID string) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}

// MoveTask moves a task to a different list
func (c *Client) MoveTask(taskID, destListID string) error {
	endpoint := fmt.Sprintf("/list/%s/task/%s", destListID, taskID)
	_, err := c.doReq("POST", endpoint, nil)
	return err
}

// UpdateAssignees replaces the assignees on a task.
func (c *Client) UpdateAssignees(taskID string, addIDs []int, removeIDs []int) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	reqBody := map[string]interface{}{}
	if len(addIDs) > 0 {
		reqBody["assignees"] = map[string]interface{}{"add": addIDs}
	}
	if len(removeIDs) > 0 {
		if _, ok := reqBody["assignees"]; ok {
			reqBody["assignees"].(map[string]interface{})["rem"] = removeIDs
		} else {
			reqBody["assignees"] = map[string]interface{}{"rem": removeIDs}
		}
	}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

// UpdateDescription updates the description of a task
func (c *Client) UpdateDescription(taskID, description string) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	reqBody := map[string]interface{}{"description": description}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}
// GetUser fetches the authenticated user's profile
func (c *Client) GetUser() (*Assignee, error) {
	data, err := c.doReq("GET", "/user", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		User Assignee `json:"user"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result.User, nil
}

func (c *Client) GetCommentReplies(commentID string) ([]Comment, error) {
	endpoint := fmt.Sprintf("/comment/%s/reply", commentID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Comments, nil
}

func (c *Client) GetTaskComments(taskID string) ([]Comment, error) {
	endpoint := fmt.Sprintf("/task/%s/comment", taskID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Comments, nil
}

// UpdateComment updates an existing comment
func (c *Client) UpdateComment(commentID, commentText string) error {
	endpoint := fmt.Sprintf("/comment/%s", commentID)
	reqBody := map[string]interface{}{"comment_text": commentText}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

// DeleteComment deletes a comment
func (c *Client) DeleteComment(commentID string) error {
	endpoint := fmt.Sprintf("/comment/%s", commentID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}

// SetTaskPriority updates the priority of a task (1: Urgent, 2: High, 3: Normal, 4: Low, nil: None)
func (c *Client) SetTaskPriority(taskID string, priority *int) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)
	
	reqBody := map[string]interface{}{
		"priority": priority,
	}
	body, _ := json.Marshal(reqBody)
	
	_, err := c.doReq("PUT", endpoint, body)
	return err
}
