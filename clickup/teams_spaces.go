package clickup

import (
	"encoding/json"
	"fmt"
)

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

func (c *Client) GetSpaceLists(spaceID string) (*SpaceHierarchy, error) {

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

func (c *Client) UpdateSpace(spaceID, name string) (*Space, error) {
	endpoint := fmt.Sprintf("/space/%s", spaceID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("PUT", endpoint, body)
	if err != nil {
		return nil, err
	}

	var space Space
	if err := json.Unmarshal(data, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

func (c *Client) DeleteSpace(spaceID string) error {
	endpoint := fmt.Sprintf("/space/%s", spaceID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
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

func (c *Client) UpdateList(listID, name string) (*List, error) {
	endpoint := fmt.Sprintf("/list/%s", listID)
	reqBody := map[string]interface{}{"name": name}
	body, _ := json.Marshal(reqBody)
	data, err := c.doReq("PUT", endpoint, body)
	if err != nil {
		return nil, err
	}

	var list List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *Client) DeleteList(listID string) error {
	endpoint := fmt.Sprintf("/list/%s", listID)
	_, err := c.doReq("DELETE", endpoint, nil)
	return err
}

func (c *Client) GetTaskMembers(taskID string) ([]Member, error) {
	endpoint := fmt.Sprintf("/task/%s/member", taskID)
	data, err := c.doReq("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Members []Member `json:"members"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Members, nil
}

func (c *Client) GetTasksForVisibleList(teamID, listID string) ([]Task, error) {
	tasks, err := c.GetTasks(listID)
	if err != nil {
		return nil, err
	}
	if teamID == "" {
		return tasks, nil
	}

	seen := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		seen[task.ID] = true
	}

	for page := 0; page < 10; page++ {
		teamTasks, err := c.GetTeamTasks(teamID, page)
		if err != nil {
			return nil, err
		}
		if len(teamTasks) == 0 {
			break
		}

		for _, task := range teamTasks {
			if seen[task.ID] {
				continue
			}

			matchesList := task.List.ID == listID
			if !matchesList {
				for _, location := range task.Locations {
					if location.ID == listID {
						matchesList = true
						break
					}
				}
			}
			if !matchesList {
				continue
			}

			seen[task.ID] = true
			tasks = append(tasks, task)
		}

		if len(teamTasks) < 100 {
			break
		}
	}

	return tasks, nil
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
