package clickup

import (
	"encoding/json"
	"fmt"
)

func buildCommentRequestBody(comment string, parts []CommentBodyPart) []byte {
	reqBody := map[string]interface{}{
		"notify_all": true,
	}
	if len(parts) > 0 {
		reqBody["comment"] = parts
	} else {
		reqBody["comment_text"] = comment
	}

	body, _ := json.Marshal(reqBody)
	return body
}

func (c *Client) AddComment(taskID, comment, parentID string, parts []CommentBodyPart) error {
	var endpoint string
	if parentID != "" {
		endpoint = fmt.Sprintf("/comment/%s/reply", parentID)
	} else {
		endpoint = fmt.Sprintf("/task/%s/comment", taskID)
	}

	body := buildCommentRequestBody(comment, parts)
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
func (c *Client) UpdateComment(commentID, commentText string, parts []CommentBodyPart) error {
	endpoint := fmt.Sprintf("/comment/%s", commentID)
	body := buildCommentRequestBody(commentText, parts)
	_, err := c.doReq("PUT", endpoint, body)
	return err
}

// DeleteComment deletes a comment
func (c *Client) DeleteComment(commentID string) error {
	endpoint := fmt.Sprintf("/comment/%s", commentID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}
