package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

func reloadProfileCmd(cfg *config.Config, width, height int, popup string) tea.Cmd {
	return func() tea.Msg { return profileReloadStartMsg{Cfg: cfg, Width: width, Height: height, Popup: popup} }
}

func loadProfileUserCmd(m *AppModel, popup string) tea.Cmd {
	return func() tea.Msg {
		u, err := m.client.GetUser()
		if err == nil {
			m.currentUser = u.Username
			m.currentUserID = u.ID
		}
		return profileReloadUserMsg{Model: m, Popup: popup, Err: err}
	}
}

func loadProfileTeamsCmd(m *AppModel, popup string) tea.Cmd {
	return func() tea.Msg {
		teams, err := m.client.GetTeams()
		if err == nil {
			m.allTeams = teams
			var items []list.Item
			for _, t := range teams {
				items = append(items, teamItem(t))
			}
			m.teamsList.SetItems(items)
		}
		return profileReloadTeamsMsg{Model: m, Popup: popup, Err: err}
	}
}

func loadProfileDefaultsCmd(m *AppModel, popup string) tea.Cmd {
	return func() tea.Msg {
		warning, err := hydrateModelDefaults(m)
		return profileReloadMsg{Model: m, Popup: popupWithWarning(popupWithErr(popup, "loading saved defaults", err), warning)}
	}
}

func fetchTeamsCmd(c *clickup.Client) tea.Cmd {
	return func() tea.Msg {
		teams, err := c.GetTeams()
		if err != nil {
			return errMsg(err)
		}
		return teamsMsg(teams)
	}
}

func fetchSpacesCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		spaces, err := c.GetSpaces(teamID)
		if err != nil {
			return errMsg(err)
		}
		return spacesMsg(spaces)
	}
}

func fetchListsCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		lists, err := c.GetSpaceLists(spaceID)
		if err != nil {
			return errMsg(err)
		}
		return listsMsg(lists)
	}
}

func createSpaceCmd(c *clickup.Client, teamID, name string) tea.Cmd {
	return func() tea.Msg {
		if _, err := c.CreateSpace(teamID, name); err != nil {
			return errMsg(err)
		}
		spaces, err := c.GetSpaces(teamID)
		if err != nil {
			return errMsg(err)
		}
		return spaceCreatedMsg{Spaces: spaces, Name: name}
	}
}

func renameSpaceCmd(c *clickup.Client, teamID, spaceID, name string) tea.Cmd {
	return func() tea.Msg {
		if _, err := c.UpdateSpace(spaceID, name); err != nil {
			return errMsg(err)
		}
		spaces, err := c.GetSpaces(teamID)
		if err != nil {
			return errMsg(err)
		}
		return spaceRenamedMsg{Spaces: spaces, Name: name}
	}
}

func deleteSpaceCmd(c *clickup.Client, teamID, spaceID, name string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteSpace(spaceID); err != nil {
			return errMsg(err)
		}
		spaces, err := c.GetSpaces(teamID)
		if err != nil {
			return errMsg(err)
		}
		return spaceDeletedMsg{Spaces: spaces, Name: name}
	}
}

func createListCmd(c *clickup.Client, spaceID, folderID, name string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if folderID != "" {
			_, err = c.CreateList(folderID, name)
		} else {
			_, err = c.CreateFolderlessList(spaceID, name)
		}
		if err != nil {
			return errMsg(err)
		}
		hierarchy, err := c.GetSpaceLists(spaceID)
		if err != nil {
			return errMsg(err)
		}
		return listCreatedMsg{Hierarchy: hierarchy, Name: name}
	}
}

func renameListCmd(c *clickup.Client, spaceID, listID, folderID, name string) tea.Cmd {
	return func() tea.Msg {
		if _, err := c.UpdateList(listID, name); err != nil {
			return errMsg(err)
		}
		hierarchy, err := c.GetSpaceLists(spaceID)
		if err != nil {
			return errMsg(err)
		}
		return listRenamedMsg{Hierarchy: hierarchy, ListID: listID, FolderID: folderID, Name: name}
	}
}

func deleteListCmd(c *clickup.Client, spaceID, listID, folderID, name string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteList(listID); err != nil {
			return errMsg(err)
		}
		hierarchy, err := c.GetSpaceLists(spaceID)
		if err != nil {
			return errMsg(err)
		}
		return listDeletedMsg{Hierarchy: hierarchy, FolderID: folderID, Name: name}
	}
}

func updateStatusCmd(c *clickup.Client, taskID, teamID, listID, status string) tea.Cmd {
	return func() tea.Msg {
		if err := c.UpdateStatus(taskID, status); err != nil {
			return errMsg(fmt.Errorf("updating status: %w", err))
		}

		task, err := c.GetTask(taskID, teamID)
		if err != nil {
			return errMsg(fmt.Errorf("reloading task after status update: %w", err))
		}

		tasks, err := c.GetTasksForVisibleList(teamID, listID)
		if err != nil {
			return errMsg(fmt.Errorf("reloading list after status update: %w", err))
		}

		comments, err := fetchCommentsRecursive(taskID, c)
		if err != nil {
			return errMsg(fmt.Errorf("reloading comments after status update: %w", err))
		}

		return statusUpdatedMsg{Task: task, Tasks: tasks, Comments: comments}
	}
}

func refreshTaskDetailCmd(c *clickup.Client, taskID, teamID string, backState state) tea.Cmd {
	return func() tea.Msg {
		task, err := c.GetTask(taskID, teamID)
		if err != nil {
			return errMsg(err)
		}
		comments, err := fetchCommentsRecursive(taskID, c)
		if err != nil {
			return errMsg(err)
		}
		return taskDetailMsg{Task: task, Comments: comments, BackState: backState}
	}
}

func fetchTasksCmd(c *clickup.Client, teamID, listID string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := c.GetTasksForVisibleList(teamID, listID)
		if err != nil {
			return errMsg(err)
		}
		return tasksMsg(tasks)
	}
}

func deleteTaskCmd(c *clickup.Client, teamID, listID, taskID, name string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteTask(taskID); err != nil {
			return errMsg(err)
		}
		tasks, err := c.GetTasksForVisibleList(teamID, listID)
		if err != nil {
			return errMsg(err)
		}
		return taskDeletedMsg{Tasks: tasks, Name: name}
	}
}

func fetchTaskCmd(c *clickup.Client, taskID, teamID string, backState state) tea.Cmd {
	return func() tea.Msg {
		task, err := c.GetTask(taskID, teamID)
		if err != nil {
			return errMsg(err)
		}

		comments, _ := fetchCommentsRecursive(task.ID, c)

		return taskDetailMsg{
			Task:      task,
			Comments:  comments,
			BackState: backState,
		}
	}
}

func fetchCommentsRecursive(taskID string, c *clickup.Client) ([]clickup.Comment, error) {
	allComments, err := c.GetTaskComments(taskID)
	if err != nil {
		return nil, err
	}

	var topLevel []clickup.Comment
	for _, c := range allComments {
		if c.Parent == nil || *c.Parent == "" {
			topLevel = append(topLevel, c)
		}
	}

	for i, j := 0, len(topLevel)-1; i < j; i, j = i+1, j-1 {
		topLevel[i], topLevel[j] = topLevel[j], topLevel[i]
	}

	for i := range topLevel {
		if topLevel[i].ReplyCount > 0 {
			replies, _ := c.GetCommentReplies(topLevel[i].ID)

			for i, j := 0, len(replies)-1; i < j; i, j = i+1, j-1 {
				replies[i], replies[j] = replies[j], replies[i]
			}
			topLevel[i].Replies = replies
		}
	}
	return flattenComments(topLevel), nil
}

func createTaskCmd(c *clickup.Client, teamID, listID, name string, userID int) tea.Cmd {
	return func() tea.Msg {
		var assignees []int
		if userID != 0 {
			assignees = []int{userID}
		}
		task, err := c.CreateTask(listID, name, assignees)
		if err != nil {
			return errMsg(err)
		}

		createdTask, err := c.GetTask(task.ID, teamID)
		if err != nil {
			return errMsg(err)
		}
		comments, err := fetchCommentsRecursive(task.ID, c)
		if err != nil {
			return errMsg(err)
		}
		tasks, err := c.GetTasksForVisibleList(teamID, listID)
		if err != nil {
			return errMsg(err)
		}

		return taskCreatedMsg{
			Task:      createdTask,
			Tasks:     tasks,
			Comments:  comments,
			BackState: stateTasks,
		}
	}
}

func createSubtaskCmd(c *clickup.Client, teamID, listID, parentID, name string, userID int) tea.Cmd {
	return func() tea.Msg {
		var assignees []int
		if userID != 0 {
			assignees = []int{userID}
		}
		if _, err := c.CreateSubtask(listID, parentID, name, assignees); err != nil {
			return errMsg(err)
		}

		parentTask, err := c.GetTask(parentID, teamID)
		if err != nil {
			return errMsg(err)
		}
		comments, err := fetchCommentsRecursive(parentID, c)
		if err != nil {
			return errMsg(err)
		}
		tasks, err := c.GetTasksForVisibleList(teamID, listID)
		if err != nil {
			return errMsg(err)
		}

		return taskCreatedMsg{
			Task:      parentTask,
			Tasks:     tasks,
			Comments:  comments,
			BackState: stateTaskDetail,
		}
	}
}

func updateDescriptionCmd(c *clickup.Client, taskID, teamID, description string, backState state) tea.Cmd {
	return func() tea.Msg {
		if err := c.UpdateDescription(taskID, description); err != nil {
			return errMsg(err)
		}
		task, err := c.GetTask(taskID, teamID)
		if err != nil {
			return errMsg(err)
		}
		comments, err := fetchCommentsRecursive(taskID, c)
		if err != nil {
			return errMsg(err)
		}
		return taskDetailMsg{
			Task:      task,
			Comments:  comments,
			BackState: backState,
		}
	}
}

func fetchAllListsForMoveCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		lists, err := c.GetSpaceLists(spaceID)
		if err != nil {
			return errMsg(err)
		}
		return moveListsReadyMsg(lists)
	}
}

func addCommentCmd(c *clickup.Client, taskID, comment, parentID string, members []clickup.Member) tea.Cmd {
	return func() tea.Msg {
		if err := c.AddComment(taskID, comment, parentID, commentPartsFromText(comment, members)); err != nil {
			return errMsg(err)
		}
		return commentAddedMsg{}
	}
}

func fetchCommentsCmd(c *clickup.Client, taskID string) tea.Cmd {
	return func() tea.Msg {
		comments, err := fetchCommentsRecursive(taskID, c)
		if err != nil {
			return errMsg(err)
		}
		return commentsMsg(comments)
	}
}

func editCommentCmd(c *clickup.Client, commentID, text string, members []clickup.Member) tea.Cmd {
	return func() tea.Msg {
		if err := c.UpdateComment(commentID, text, commentPartsFromText(text, members)); err != nil {
			return errMsg(err)
		}
		return commentAddedMsg{}
	}
}

func setPriorityCmd(c *clickup.Client, taskID, teamID string, priority *int) tea.Cmd {
	return func() tea.Msg {
		if err := c.SetTaskPriority(taskID, priority); err != nil {
			return errMsg(err)
		}

		task, _ := c.GetTask(taskID, teamID)
		comments, _ := fetchCommentsRecursive(taskID, c)
		return taskDetailMsg{Task: task, Comments: comments}
	}
}

func deleteCommentCmd(c *clickup.Client, commentID string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteComment(commentID); err != nil {
			return errMsg(err)
		}
		return commentDeletedMsg{}
	}
}

func openAttachmentURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", url).Run()
		return nil
	}
}

func uploadAttachmentCmd(c *clickup.Client, taskID, teamID, sourcePath string, backState state, popup string, cleanup func()) tea.Cmd {
	return func() tea.Msg {
		if cleanup != nil {
			defer cleanup()
		}

		if err := c.UploadTaskAttachment(taskID, sourcePath); err != nil {
			return errMsg(err)
		}

		task, err := c.GetTask(taskID, teamID)
		if err != nil {
			return errMsg(err)
		}
		comments, err := fetchCommentsRecursive(taskID, c)
		if err != nil {
			return errMsg(err)
		}
		return attachmentUploadedMsg{Task: task, Comments: comments, BackState: backState, Popup: popup}
	}
}

func (m *AppModel) copyTaskDescription() tea.Cmd {
	desc := m.editableDescription()
	clipboard.WriteAll(desc)
	m.popupMsg = "Copied description to clipboard"
	return tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
		return clearPopupMsg{}
	})
}

func (m *AppModel) copyTaskTitle() tea.Cmd {
	clipboard.WriteAll(m.selectedTask.Name)
	m.popupMsg = "Copied title to clipboard"
	return tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
		return clearPopupMsg{}
	})
}

func (m *AppModel) copyTaskChecklists() tea.Cmd {
	var sb strings.Builder
	if len(m.selectedTask.Checklists) > 0 {
		for _, cl := range m.selectedTask.Checklists {
			sb.WriteString(fmt.Sprintf("- %s\n", cl.Name))
			for _, item := range cl.Items {
				status := "[ ]"
				if item.Resolved {
					status = "[x]"
				}
				sb.WriteString(fmt.Sprintf("  %s %s\n", status, item.Name))
			}
		}
	} else {
		m.popupMsg = "No checklists to copy"
		return tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
			return clearPopupMsg{}
		})
	}
	clipboard.WriteAll(sb.String())
	m.popupMsg = "Copied checklists to clipboard"
	return tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
		return clearPopupMsg{}
	})
}

func (m *AppModel) copyTaskForAI() tea.Cmd {
	var sb strings.Builder
	t := m.selectedTask
	ticket_id := t.ID
	if t.CustomID != "" {
		ticket_id = t.CustomID
	}

	sb.WriteString(fmt.Sprintf("Ticket ID: %s\n", ticket_id))
	sb.WriteString(fmt.Sprintf("Title: %s\n", t.Name))

	desc := m.editableDescription()
	if desc != "" {
		sb.WriteString("\n=== Description ===\n")
		sb.WriteString(desc)
		sb.WriteString("\n")
	}

	if len(t.Checklists) > 0 {
		sb.WriteString("\n=== Checklists ===\n")
		for _, cl := range t.Checklists {
			sb.WriteString(fmt.Sprintf("- %s\n", cl.Name))
			for _, item := range cl.Items {
				status := "[ ]"
				if item.Resolved {
					status = "[x]"
				}
				sb.WriteString(fmt.Sprintf("  %s %s\n", status, item.Name))
			}
		}
	}

	if len(m.selectedComments) > 0 {
		sb.WriteString("\n=== Comments ===\n")
		for _, c := range m.selectedComments {
			dateStr := c.Date
			if ms, err := strconv.ParseInt(c.Date, 10, 64); err == nil {
				dateStr = time.UnixMilli(ms).Format(time.RFC822)
			}
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n-----------\n", c.User.Username, dateStr, c.CommentText))
		}
	}

	clipboard.WriteAll(sb.String())
	m.popupMsg = "Copied task context for AI to clipboard"
	return tea.Tick(time.Second*2, func(_ time.Time) tea.Msg {
		return clearPopupMsg{}
	})
}

func fetchTaskMembersCmd(c *clickup.Client, taskID string) tea.Cmd {
	return func() tea.Msg {
		members, err := c.GetTaskMembers(taskID)
		if err != nil {
			return errMsg(err)
		}
		return teamMembersMsg(members)
	}
}

func searchTasksCmd(c *clickup.Client, teamID, query string) tea.Cmd {
	return func() tea.Msg {
		if teamID == "" {
			return errMsg(fmt.Errorf("global search requires a workspace/team to be selected or configured"))
		}

		parsed := parseSearchQuery(query)
		seen := make(map[string]bool)
		var matches []clickup.Task
		for page := 0; page < 10; page++ {
			tasks, err := c.GetTeamTasks(teamID, page)
			if err != nil {
				return errMsg(err)
			}
			if len(tasks) == 0 {
				break
			}
			for _, task := range tasks {
				if seen[task.ID] {
					continue
				}
				seen[task.ID] = true
				if !matchesSearchFilters(parsed, task) {
					continue
				}
				if scoreSearchMatch(parsed, task) > 0 {
					matches = append(matches, task)
				}
			}
			if len(tasks) < 100 {
				break
			}
		}

		sort.SliceStable(matches, func(i, j int) bool {
			si := scoreSearchMatch(parsed, matches[i])
			sj := scoreSearchMatch(parsed, matches[j])
			if si != sj {
				return si > sj
			}
			idi := matches[i].ID
			if matches[i].CustomID != "" {
				idi = matches[i].CustomID
			}
			idj := matches[j].ID
			if matches[j].CustomID != "" {
				idj = matches[j].CustomID
			}
			return idi < idj
		})

		if len(matches) > 100 {
			matches = matches[:100]
		}

		return searchResultsMsg{Query: parsed.Raw, Tasks: matches}
	}
}

func openExternalEditorCmd(initialContent string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	tmp, err := os.CreateTemp("", "clickup-desc-*.md")
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	tmp.WriteString(initialContent)
	tmpPath := tmp.Name()
	tmp.Close()

	c := exec.Command(editor, tmpPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return editorFinishedMsg{err: err}
		}
		data, readErr := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if readErr != nil {
			return editorFinishedMsg{err: readErr}
		}
		return editorFinishedMsg{content: string(data)}
	})
}
