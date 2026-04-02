package clickup

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	var allTasks []Task
	for page := 0; ; page++ {
		endpoint := fmt.Sprintf("/list/%s/task?page=%d&subtasks=true&include_closed=true&include_markdown_description=true&include_timl=true", listID, page)
		data, err := c.doReq("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var result struct {
			Tasks    []Task `json:"tasks"`
			LastPage bool   `json:"last_page"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		for i := range result.Tasks {
			normalizeTask(&result.Tasks[i])
		}
		allTasks = append(allTasks, result.Tasks...)
		if result.LastPage || len(result.Tasks) == 0 {
			break
		}
	}
	return allTasks, nil
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

func (c *Client) CreateChecklist(taskID, name string) error {
	endpoint := fmt.Sprintf("/task/%s/checklist", taskID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("POST", endpoint, body)
	return err
}

func (c *Client) UpdateChecklist(checklistID, name string) error {
	endpoint := fmt.Sprintf("/checklist/%s", checklistID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

func (c *Client) DeleteChecklist(checklistID string) error {
	endpoint := fmt.Sprintf("/checklist/%s", checklistID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}

func (c *Client) CreateChecklistItem(checklistID, name string) error {
	endpoint := fmt.Sprintf("/checklist/%s/checklist_item", checklistID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("POST", endpoint, body)
	return err
}

func (c *Client) UpdateChecklistItem(checklistID, itemID, name string, resolved bool, parentID *string) error {
	endpoint := fmt.Sprintf("/checklist/%s/checklist_item/%s", checklistID, itemID)
	reqBody := map[string]interface{}{
		"name":     name,
		"resolved": resolved,
	}
	if parentID != nil {
		if *parentID == "" {
			reqBody["parent"] = nil
		} else {
			reqBody["parent"] = *parentID
		}
	}
	body, _ := json.Marshal(reqBody)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

func (c *Client) DeleteChecklistItem(checklistID, itemID string) error {
	endpoint := fmt.Sprintf("/checklist/%s/checklist_item/%s", checklistID, itemID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}

func (c *Client) UpdateTaskName(taskID, name string) error {
	endpoint := fmt.Sprintf("/task/%s", taskID)

	reqBody := map[string]interface{}{
		"name": name,
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

func (c *Client) UploadTaskAttachment(taskID, sourcePath string) error {
	endpoint := fmt.Sprintf("/task/%s/attachment", taskID)
	_, err := c.uploadFileReq(endpoint, "attachment", sourcePath)
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
