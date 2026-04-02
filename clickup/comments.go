package clickup

import (
	"encoding/json"
	"fmt"
)

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
