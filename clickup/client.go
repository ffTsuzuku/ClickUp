package clickup

import (
	"fmt"
	"time"
)

type TaskStatus struct {
	Status string
	Color  string
}

type Comment struct {
	ID        string
	Comment   string
	Date      int64
	User      string
}

type Task struct {
	ID        string
	Name      string
	Desc      string
	Status    TaskStatus
	Assignee  string
	Comments  []Comment
}

// Client represents the ClickUp API client
type Client struct {
	Token string
	// Mock tasks for now
	Tasks []Task
}

func NewClient(token string) *Client {
	return &Client{
		Token: token,
		Tasks: []Task{
			{
				ID:          "1abc",
				Name:        "Design new TUI layout",
				Desc:        "We need a beautiful, Gemini-like TUI for our users. Include views for lists and task details.",
				Status:      TaskStatus{Status: "in progress", Color: "#f1e05a"},
				Assignee:    "Tsuzuku",
				Comments: []Comment{
					{ID: "c1", Comment: "Starting this right now using Bubble Tea.", Date: time.Now().UnixMilli(), User: "Tsuzuku"},
				},
			},
			{
				ID:          "2xyz",
				Name:        "Implement API authentication",
				Desc:        "Read the API token from env var and pass it in Authorization header.",
				Status:      TaskStatus{Status: "to do", Color: "#8b949e"},
				Assignee:    "Tsuzuku",
			},
			{
				ID:          "3lmn",
				Name:        "Fix rendering bug in lipgloss",
				Desc:        "Borders are drawing over text in the detail view.",
				Status:      TaskStatus{Status: "done", Color: "#2ea043"},
				Assignee:    "Tsuzuku",
				Comments: []Comment{
					{ID: "c2", Comment: "Fixed in latest commit.", Date: time.Now().UnixMilli(), User: "Tsuzuku"},
				},
			},
		},
	}
}

// GetTasks fetches tasks (mocked for now)
func (c *Client) GetTasks() ([]Task, error) {
	return c.Tasks, nil
}

// AddComment adds a comment to a task (mocked)
func (c *Client) AddComment(taskID, comment string) error {
	for i, t := range c.Tasks {
		if t.ID == taskID {
			c.Tasks[i].Comments = append(t.Comments, Comment{
				ID:      fmt.Sprintf("cmd_mock_%d", time.Now().UnixMilli()),
				Comment: comment,
				Date:    time.Now().UnixMilli(),
				User:    "Tsuzuku",
			})
			return nil
		}
	}
	return fmt.Errorf("task not found")
}

// UpdateStatus updates task status
func (c *Client) UpdateStatus(taskID, status string) error {
	for i, t := range c.Tasks {
		if t.ID == taskID {
			c.Tasks[i].Status.Status = status
			return nil
		}
	}
	return fmt.Errorf("task not found")
}
