package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

func (c *Client) uploadFileReq(endpoint, fieldName, sourcePath string) ([]byte, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filepath.Base(sourcePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", BaseURL+endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

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

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Space struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Statuses []TaskStatus `json:"statuses"`
}

type Folder struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Lists []List `json:"lists"`
}

type List struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskLocation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpaceHierarchy struct {
	Folders []Folder `json:"folders"`
	Lists   []List   `json:"lists"`
}

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

type CommentBodyPart struct {
	Text       string                 `json:"text,omitempty"`
	Type       string                 `json:"type,omitempty"`
	User       *CommentTaggedUser     `json:"user,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type CommentTaggedUser struct {
	ID int `json:"id"`
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
	ID           string `json:"id"`
	Title        string `json:"title"`
	Extension    string `json:"extension"`
	URL          string `json:"url"`
	URLWithQuery string `json:"url_w_query"`
}

type ChecklistItem struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Resolved    bool            `json:"resolved"`
	OrderIndex  *int            `json:"orderindex"`
	DateCreated string          `json:"date_created"`
	Parent      *string         `json:"parent,omitempty"`
	Children    []ChecklistItem `json:"children,omitempty"`
}

type Checklist struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Items []ChecklistItem `json:"items"`
}

type Task struct {
	ID                  string         `json:"id"`
	CustomID            string         `json:"custom_id"`
	Name                string         `json:"name"`
	Desc                string         `json:"description"`
	Status              TaskStatus     `json:"status"`
	Assignees           []Assignee     `json:"assignees"`
	URL                 string         `json:"url"`
	Points              *float64       `json:"points"`
	Priority            *Priority      `json:"priority,omitempty"`
	Parent              *string        `json:"parent,omitempty"`
	Attachments         []Attachment   `json:"attachments"`
	Checklists          []Checklist    `json:"checklists"`
	DateCreated         string         `json:"date_created"`
	Creator             Assignee       `json:"creator"`
	MarkdownDescription string         `json:"markdown_description"`
	List                List           `json:"list"`
	Locations           []TaskLocation `json:"locations"`
}

var escapedMarkdownCharRE = regexp.MustCompile(`\\+([*_` + "`" + `~\[\]\(\)#>!\-\+\|])`)

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
