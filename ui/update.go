package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}

		s := msg.String()
		isEditingState := m.state == stateCommand ||
			m.state == stateComment ||
			m.state == stateEditDesc ||
			m.state == stateCreateTask ||
			m.state == stateCreateSubtask ||
			m.state == stateEditComment

		if (s == "/" || s == ":") && !isEditingState {
			m.prevState = m.state
			m.state = stateCommand
			m.cmdInput.Focus()
			m.cmdInput.SetValue(s)
			m.cmdInput.SetCursor(len(m.cmdInput.Value()))
			m.updateCommandSuggestions()
			m.updateLayout()
			return m, textinput.Blink
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
	case spacesMsg:
		m.loading = false
		m.allSpaces = msg
		var items []list.Item
		for _, s := range msg {
			items = append(items, spaceItem(s))
		}
		m.spacesList.SetItems(items)
		m.state = stateSpaces
		m.activeList = &m.spacesList
		return m, nil
	case listsMsg:
		m.loading = false
		m.allFolders = msg.Folders
		m.allLists = msg.Lists
		m.selectedFolder = nil
		var items []list.Item
		for _, f := range m.allFolders {
			items = append(items, folderItem(f))
		}
		for _, l := range m.allLists {
			items = append(items, listItem(l))
		}
		m.listsList.SetItems(items)
		m.state = stateLists
		m.activeList = &m.listsList
		return m, nil
	case tasksMsg:
		m.loading = false
		m.allTasks = msg
		m.taskHistory = nil
		m.applyTaskFilter("")
		m.state = stateTasks
		m.activeList = &m.tasksList
		return m, nil
	case searchResultsMsg:
		m.loading = false
		m.searchResults = msg.Tasks
		m.searchQuery = msg.Query
		var items []list.Item
		for _, t := range msg.Tasks {
			items = append(items, taskItem(t))
		}
		m.searchList.Title = fmt.Sprintf("Search Results (%d): %s", len(msg.Tasks), msg.Query)
		m.searchList.SetItems(items)
		m.searchList.Select(0)
		m.state = stateSearchResults
		m.activeList = &m.searchList
		return m, nil
	case taskDetailMsg:
		return m.applyTaskDetail(msg.Task, msg.Comments, msg.BackState, false)
	case attachmentUploadedMsg:
		_, cmd := m.applyTaskDetail(msg.Task, msg.Comments, msg.BackState, true)
		m.popupMsg = msg.Popup
		return m, tea.Batch(cmd, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }))
	case taskCreatedMsg:
		m.allTasks = msg.Tasks
		m.applyTaskFilter("")
		preserveHistory := msg.BackState == stateTaskDetail && msg.Task != nil && msg.Task.ID == m.selectedTask.ID
		return m.applyTaskDetail(msg.Task, msg.Comments, msg.BackState, preserveHistory)
	case taskDeletedMsg:
		m.loading = false
		m.allTasks = msg.Tasks
		m.pendingDeleteTaskID = ""
		m.pendingDeleteTaskName = ""
		m.applyTaskFilter("")
		m.state = stateTasks
		m.activeList = &m.tasksList
		m.popupMsg = "Deleted task " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case moveListsReadyMsg:
		m.loading = false
		// Flatten for move picker (simpler for now)
		var flat []clickup.List
		for _, f := range msg.Folders {
			flat = append(flat, f.Lists...)
		}
		flat = append(flat, msg.Lists...)
		m.moveCandidateLists = flat
		m.state = stateMovePicker
		return m, nil
	case teamMembersMsg:
		m.loading = false
		m.teamMembers = []clickup.Member(msg)
		m.teamMembersTaskID = m.selectedTask.ID
		m.refreshMentionSuggestions()
		return m, nil
	case teamsMsg:
		m.loading = false
		var items []list.Item
		m.allTeams = []clickup.Team(msg)
		for _, t := range m.allTeams {
			items = append(items, teamItem(t))
		}
		m.teamsList.SetItems(items)
		return m, nil
	case spaceCreatedMsg:
		m.loading = false
		m.allSpaces = msg.Spaces
		var items []list.Item
		for _, s := range msg.Spaces {
			items = append(items, spaceItem(s))
		}
		m.spacesList.SetItems(items)
		m.state = stateSpaces
		m.activeList = &m.spacesList
		m.popupMsg = "Created space " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case spaceRenamedMsg:
		m.loading = false
		m.allSpaces = msg.Spaces
		var items []list.Item
		for _, s := range msg.Spaces {
			items = append(items, spaceItem(s))
		}
		m.spacesList.SetItems(items)
		m.state = stateSpaces
		m.activeList = &m.spacesList
		m.popupMsg = "Renamed space to " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case spaceDeletedMsg:
		m.loading = false
		m.allSpaces = msg.Spaces
		m.selectedSpace = ""
		m.pendingDeleteSpaceID = ""
		m.pendingDeleteSpaceName = ""
		var items []list.Item
		for _, s := range msg.Spaces {
			items = append(items, spaceItem(s))
		}
		m.spacesList.SetItems(items)
		m.state = stateSpaces
		m.activeList = &m.spacesList
		m.popupMsg = "Deleted space " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case listCreatedMsg:
		m.loading = false
		m.allFolders = msg.Hierarchy.Folders
		m.allLists = msg.Hierarchy.Lists
		m.selectedFolder = nil
		var items []list.Item
		for _, f := range m.allFolders {
			items = append(items, folderItem(f))
		}
		for _, l := range m.allLists {
			items = append(items, listItem(l))
		}
		m.listsList.SetItems(items)
		m.state = stateLists
		m.activeList = &m.listsList
		m.popupMsg = "Created list " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case listRenamedMsg:
		m.loading = false
		m.allFolders = msg.Hierarchy.Folders
		m.allLists = msg.Hierarchy.Lists
		m.selectedFolder = nil

		var items []list.Item
		if msg.FolderID != "" {
			for _, f := range m.allFolders {
				if f.ID == msg.FolderID {
					folder := f
					m.selectedFolder = &folder
					for _, l := range f.Lists {
						items = append(items, listItem(l))
					}
					break
				}
			}
		}
		if len(items) == 0 {
			for _, f := range m.allFolders {
				items = append(items, folderItem(f))
			}
			for _, l := range m.allLists {
				items = append(items, listItem(l))
			}
		}
		m.listsList.SetItems(items)
		m.state = stateLists
		m.activeList = &m.listsList
		m.popupMsg = "Renamed list to " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case listDeletedMsg:
		m.loading = false
		m.allFolders = msg.Hierarchy.Folders
		m.allLists = msg.Hierarchy.Lists
		m.selectedFolder = nil
		m.selectedList = ""
		m.pendingDeleteListID = ""
		m.pendingDeleteListName = ""
		m.pendingDeleteListFolderID = ""

		var items []list.Item
		if msg.FolderID != "" {
			for _, f := range m.allFolders {
				if f.ID == msg.FolderID {
					folder := f
					m.selectedFolder = &folder
					for _, l := range f.Lists {
						items = append(items, listItem(l))
					}
					break
				}
			}
		}
		if len(items) == 0 {
			for _, f := range m.allFolders {
				items = append(items, folderItem(f))
			}
			for _, l := range m.allLists {
				items = append(items, listItem(l))
			}
		}
		m.listsList.SetItems(items)
		m.state = stateLists
		m.activeList = &m.listsList
		m.popupMsg = "Deleted list " + msg.Name
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case statusUpdatedMsg:
		m.loading = false
		m.selectedTask = *msg.Task
		m.allTasks = msg.Tasks
		m.selectedComments = msg.Comments
		m.updateViewportContent()
		m.applyHierarchyFilter(strings.TrimPrefix(m.cmdInput.Value(), "/filter "))
		m.popupMsg = "Updated status to " + m.selectedTask.Status.Status
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case editorFinishedMsg:
		if msg.err == nil && msg.content != "" {
			content := strings.TrimRight(msg.content, "\n")
			if m.externalEditTarget == "description" || m.state == stateEditDesc {
				m.loading = true
				m.loadingMsg = "Updating description..."
				cmds = append(cmds, tea.Batch(m.spinner.Tick, updateDescriptionCmd(m.client, m.selectedTask.ID, m.selectedTeam, content, m.detailBackState)))
			} else if m.externalEditTarget == "comment" || m.state == stateEditComment {
				m.loading = true
				m.loadingMsg = "Updating comment..."
				cmds = append(cmds, tea.Batch(m.spinner.Tick, editCommentCmd(m.client, m.editingCommentID, content, m.mentionableMembers())))
			} else if m.externalEditTarget == "new_comment" || m.state == stateComment {
				m.loading = true
				m.loadingMsg = "Adding comment..."
				cmds = append(cmds, tea.Batch(m.spinner.Tick, addCommentCmd(m.client, m.selectedTask.ID, content, m.replyToCommentID, m.mentionableMembers())))
			}
		}
		m.externalEditTarget = ""
		if m.state == stateComment || m.state == stateEditComment {
			m.state = m.commentReturnState
		} else {
			m.state = stateTaskDetail
		}
		m.updateViewportContent()
		return m, tea.Batch(cmds...)
	case commentAddedMsg:
		m.replyToCommentID = ""
		m.replyToUser = ""
		m.refreshMentionSuggestions()
		m.popupMsg = "Comment added!"
		m.state = m.commentReturnState
		return m, tea.Batch(
			fetchCommentsCmd(m.client, m.selectedTask.ID),
			tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
		)
	case commentDeletedMsg:
		m.replyToCommentID = ""
		m.replyToUser = ""
		m.refreshMentionSuggestions()
		m.popupMsg = "Comment deleted!"
		return m, tea.Batch(
			fetchCommentsCmd(m.client, m.selectedTask.ID),
			tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
		)

	case checklistItemUpdatedMsg:
		m.loading = false
		return m, tea.Batch(
			fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState),
		)

	case checklistDeletedMsg:
		m.loading = false
		return m, tea.Batch(
			fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState),
		)
	case commentsMsg:
		m.loading = false
		m.selectedComments = msg
		if len(m.selectedComments) == 0 {
			m.commentSelectedIdx = 0
		} else if m.commentSelectedIdx >= len(m.selectedComments) {
			m.commentSelectedIdx = len(m.selectedComments) - 1
		}
		if m.state == stateCommentsView || m.commentReturnState == stateCommentsView {
			m.updateCommentsViewportContent()
		} else {
			m.updateViewportContent()
		}
		return m, nil
	case errMsg:
		m.loading = false
		m.err = msg
		m.recordError("application error", msg)
		m.popupMsg = "Error: " + msg.Error()
		return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case clearPopupMsg:
		m.popupMsg = ""
		return m, nil
	case profileReloadStartMsg:
		base := newBaseModel(msg.Cfg)
		base.width = msg.Width
		base.height = msg.Height
		base.updateLayout()
		if msg.Cfg.ClickupAPIKey == "" || msg.Cfg.ClickupAPIKey == "NO_TOKEN" {
			base.popupMsg = msg.Popup
			*m = *base
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
		m.loadingMsg = "Authenticating with ClickUp..."
		return m, loadProfileUserCmd(base, msg.Popup)
	case profileReloadUserMsg:
		if msg.Err != nil {
			msg.Model.recordError("profile reload: authenticating profile", msg.Err)
			msg.Model.popupMsg = popupWithErr(msg.Popup, "authenticating profile", msg.Err)
			*m = *msg.Model
			return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
		m.loadingMsg = "Loading workspaces..."
		return m, loadProfileTeamsCmd(msg.Model, msg.Popup)
	case profileReloadTeamsMsg:
		if msg.Err != nil {
			msg.Model.recordError("profile reload: loading workspaces", msg.Err)
			msg.Model.popupMsg = popupWithErr(msg.Popup, "loading workspaces", msg.Err)
			*m = *msg.Model
			return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
		m.loadingMsg = "Loading saved defaults..."
		return m, loadProfileDefaultsCmd(msg.Model, msg.Popup)
	case profileReloadMsg:
		msg.Model.popupMsg = msg.Popup
		*m = *msg.Model
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	switch m.state {
	case stateTeams, stateSpaces, stateLists, stateTasks, stateSearchResults, stateFilePicker:
		return m.updateList(msg)
	case stateTaskDetail:
		return m.updateDetail(msg)
	case stateComment:
		return m.updateComment(msg)
	case stateCommand:
		return m.updateCommand(msg)
	case stateHelp:
		return m.updateHelp(msg)
	case stateCreateTask:
		return m.updateCreateTask(msg)
	case stateMovePicker:
		return m.updateMovePicker(msg)
	case stateEditTitle:
		return m.updateEditTitle(msg)
	case stateEditDesc:
		return m.updateEditDesc(msg)
	case stateCreateSubtask:
		return m.updateCreateSubtask(msg)
	case stateEditComment:
		return m.updateEditComment(msg)
	case stateConfirmTaskDelete:
		return m.updateConfirmTaskDelete(msg)
	case stateChecklist:
		return m.updateChecklist(msg)
	case stateConfirmChecklistDelete:
		return m.updateConfirmChecklistDelete(msg)
	case stateCommentsView:
		return m.updateCommentsView(msg)
	case stateConfirmCommentDelete:
		return m.updateConfirmCommentDelete(msg)
	case stateConfirmProfileDelete:
		return m.updateConfirmProfileDelete(msg)
	case stateConfirmListDelete:
		return m.updateConfirmListDelete(msg)
	case stateConfirmSpaceDelete:
		return m.updateConfirmSpaceDelete(msg)
	case stateConfirmDiscardDesc:
		return m.updateConfirmDiscardDesc(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			if m.state == stateFilePicker {
				m.state = stateTaskDetail
			} else if m.state == stateTasks {
				m.state = stateLists
				m.activeList = &m.listsList
			} else if m.state == stateSearchResults {
				m.state = stateTasks
				m.activeList = &m.tasksList
			} else if m.state == stateLists {
				if m.selectedFolder != nil {
					m.selectedFolder = nil
					var items []list.Item
					for _, f := range m.allFolders {
						items = append(items, folderItem(f))
					}
					for _, l := range m.allLists {
						items = append(items, listItem(l))
					}
					m.listsList.SetItems(items)
					return m, nil
				}
				m.state = stateSpaces
				m.activeList = &m.spacesList
			} else if m.state == stateSpaces {
				m.state = stateTeams
				m.activeList = &m.teamsList
			}
			return m, nil
		case ".", "H":
			if m.state == stateFilePicker {
				m.filePickerShowHidden = !m.filePickerShowHidden
				if err := m.openFilePicker(m.filePickerPath); err != nil {
					m.popupMsg = "Error: " + err.Error()
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if m.filePickerShowHidden {
					m.popupMsg = "Showing hidden files"
				} else {
					m.popupMsg = "Hiding hidden files"
				}
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			return m, nil
		case "a", "n":
			if m.state == stateTasks {
				m.state = stateCreateTask
				m.taskInput.SetValue("")
				m.taskInput.Focus()
				return m, textinput.Blink
			} else if m.state == stateSpaces {
				m.prevState = stateSpaces
				m.state = stateCommand
				m.cmdInput.Focus()
				m.cmdInput.SetValue("/space create ")
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.updateCommandSuggestions()
				m.updateLayout()
				return m, textinput.Blink
			} else if m.state == stateLists {
				m.prevState = stateLists
				m.state = stateCommand
				m.cmdInput.Focus()
				m.cmdInput.SetValue("/list create ")
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.updateCommandSuggestions()
				m.updateLayout()
				return m, textinput.Blink
			}
		case "e":
			if m.state == stateSpaces {
				m.prevState = stateSpaces
				m.state = stateCommand
				m.cmdInput.Focus()
				m.cmdInput.SetValue("/space rename ")
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.updateCommandSuggestions()
				m.updateLayout()
				return m, textinput.Blink
			} else if m.state == stateLists {
				selected, ok := m.activeList.SelectedItem().(listItem)
				if !ok {
					m.popupMsg = "Error: highlight a list to rename"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.prevState = stateLists
				m.state = stateCommand
				m.cmdInput.Focus()
				m.cmdInput.SetValue("/list rename " + selected.Name)
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.updateCommandSuggestions()
				m.updateLayout()
				return m, textinput.Blink
			}
		case "d":
			if m.state == stateSpaces {
				selected, ok := m.activeList.SelectedItem().(spaceItem)
				if !ok {
					m.popupMsg = "Error: highlight a space to delete"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteSpaceID = selected.ID
				m.pendingDeleteSpaceName = selected.Name
				m.prevState = m.state
				m.state = stateConfirmSpaceDelete
				return m, nil
			} else if m.state == stateLists {
				selected, ok := m.activeList.SelectedItem().(listItem)
				if !ok {
					m.popupMsg = "Error: highlight a list to delete"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteListID = selected.ID
				m.pendingDeleteListName = selected.Name
				m.pendingDeleteListFolderID = ""
				if m.selectedFolder != nil {
					m.pendingDeleteListFolderID = m.selectedFolder.ID
				}
				m.prevState = m.state
				m.state = stateConfirmListDelete
				return m, nil
			} else if m.state == stateTasks {
				selected, ok := m.activeList.SelectedItem().(taskItem)
				if !ok {
					m.popupMsg = "Error: highlight a task to delete"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteTaskID = selected.ID
				m.pendingDeleteTaskName = selected.Name
				m.prevState = m.state
				m.state = stateConfirmTaskDelete
				return m, nil
			}
		case "o":
			if m.state == stateTasks {
				if i, ok := m.activeList.SelectedItem().(taskItem); ok {
					if i.URL != "" {
						m.popupMsg = "Opening in Browser..."
						return m, tea.Batch(
							openAttachmentURLCmd(i.URL),
							tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
						)
					}
				}
			}
		case "r":
			m.loading = true
			switch m.state {
			case stateTeams:
				m.loadingMsg = "Refreshing teams..."
				return m, tea.Batch(m.spinner.Tick, fetchTeamsCmd(m.client))
			case stateSpaces:
				m.loadingMsg = "Refreshing spaces..."
				return m, tea.Batch(m.spinner.Tick, fetchSpacesCmd(m.client, m.selectedTeam))
			case stateLists:
				m.loadingMsg = "Refreshing lists..."
				return m, tea.Batch(m.spinner.Tick, fetchListsCmd(m.client, m.selectedSpace))
			case stateTasks:
				m.loadingMsg = "Refreshing tasks..."
				return m, tea.Batch(m.spinner.Tick, fetchTasksCmd(m.client, m.selectedTeam, m.selectedList))
			}
		case "enter", "right":
			switch m.state {
			case stateTeams:
				if i, ok := m.activeList.SelectedItem().(teamItem); ok {
					m.selectedTeam = i.ID
					m.teamMembers = nil
					m.teamMembersTaskID = ""
					m.loading = true
					m.loadingMsg = "Loading spaces..."
					return m, tea.Batch(m.spinner.Tick, fetchSpacesCmd(m.client, m.selectedTeam))
				}
			case stateSpaces:
				if i, ok := m.activeList.SelectedItem().(spaceItem); ok {
					m.selectedSpace = i.ID
					m.loading = true
					m.loadingMsg = "Loading lists..."
					return m, tea.Batch(m.spinner.Tick, fetchListsCmd(m.client, m.selectedSpace))
				}
			case stateLists:
				if i, ok := m.activeList.SelectedItem().(folderItem); ok {
					folder := clickup.Folder(i)
					m.selectedFolder = &folder
					var items []list.Item
					for _, l := range folder.Lists {
						items = append(items, listItem(l))
					}
					m.listsList.SetItems(items)
					m.activeList.Select(0)
					return m, nil
				}
				if i, ok := m.activeList.SelectedItem().(listItem); ok {
					m.selectedList = i.ID
					m.loading = true
					m.loadingMsg = "Loading tasks..."
					return m, tea.Batch(m.spinner.Tick, fetchTasksCmd(m.client, m.selectedTeam, m.selectedList))
				}
			case stateTasks:
				if i, ok := m.activeList.SelectedItem().(taskItem); ok {
					m.selectedTask = clickup.Task(i)
					m.taskHistory = nil
					m.loading = true
					m.loadingMsg = "Fetching task details..."
					return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, i.ID, m.selectedTeam, stateTasks))
				}
			case stateSearchResults:
				if i, ok := m.activeList.SelectedItem().(taskItem); ok {
					m.selectedTask = clickup.Task(i)
					m.taskHistory = nil
					m.loading = true
					m.loadingMsg = "Fetching task details..."
					return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, i.ID, m.selectedTeam, stateSearchResults))
				}
			case stateFilePicker:
				if i, ok := m.activeList.SelectedItem().(filePickerItem); ok {
					if i.IsDir {
						if err := m.openFilePicker(i.Path); err != nil {
							m.popupMsg = "Error: " + err.Error()
							return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
						}
						return m, nil
					}
					m.loading = true
					m.loadingMsg = "Uploading attachment..."
					return m, tea.Batch(m.spinner.Tick, uploadAttachmentCmd(m.client, m.selectedTask.ID, m.selectedTeam, i.Path, m.prevState, "Uploaded attachment", nil))
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	*m.activeList, cmd = m.activeList.Update(msg)
	return m, cmd
}

func (m *AppModel) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "left":
			if len(m.taskHistory) > 0 {
				m.selectedTask = m.taskHistory[len(m.taskHistory)-1]
				m.taskHistory = m.taskHistory[:len(m.taskHistory)-1]
				m.updateViewportContent()
			} else {
				if m.prevState == stateSearchResults {
					m.state = stateSearchResults
					m.activeList = &m.searchList
				} else {
					m.state = stateTasks
					m.activeList = &m.tasksList
				}
			}
			return m, nil
		case "c":
			m.commentReturnState = m.state
			m.state = stateComment
			m.commentInput.SetValue("")
			m.commentInput.Focus()
			m.refreshMentionSuggestions()
			return m, textinput.Blink
		case "A":
			return m, m.copyTaskForAI()
		case "a":
			m.parentTaskID = m.selectedTask.ID
			m.state = stateCreateSubtask
			m.taskInput.SetValue("")
			m.taskInput.Focus()
			return m, textinput.Blink
		case "s":
			if m.selectedTask.URL != "" {
				clipboard.WriteAll(m.selectedTask.URL)
				m.popupMsg = "Copied URL to Clipboard"
				return m, tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
					return clearPopupMsg{}
				})
			}
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			subtasks := m.getSubtasks(m.selectedTask.ID)
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(subtasks) {
				m.taskHistory = append(m.taskHistory, m.selectedTask)
				m.loading = true
				m.loadingMsg = "Fetching subtask details..."
				return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, subtasks[idx].ID, m.selectedTeam, stateTaskDetail))
			}
			return m, nil
		case "o":
			if m.selectedTask.URL != "" {
				m.popupMsg = "Opening in Browser..."
				return m, tea.Batch(
					openAttachmentURLCmd(m.selectedTask.URL),
					tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
				)
			}
			return m, nil
		case "r":
			m.loading = true
			m.loadingMsg = "Refreshing task..."
			return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.prevState))
		case "L":
			if len(m.selectedTask.Checklists) > 0 {
				m.flattenChecklists()
				m.checklistSelectedIdx = 0
				m.checklistEditingItem = nil
				m.state = stateChecklist
				m.updateChecklistViewportContent()
				return m, nil
			}
			m.popupMsg = "No checklists on this task. Press 'n' from command mode to create one."
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		case "C":
			m.commentSelectedIdx = 0
			m.state = stateCommentsView
			m.updateCommentsViewportContent()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateComment(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.handleMentionKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+s":
			v := m.commentInput.Value()
			if v != "" {
				m.commentInput.SetValue("")
				m.commentInput.Blur()
				m.loading = true
				m.loadingMsg = "Adding comment..."
				return m, tea.Batch(m.spinner.Tick, addCommentCmd(m.client, m.selectedTask.ID, v, m.replyToCommentID, m.mentionableMembers()))
			}
			return m, nil
		case "ctrl+e":
			m.externalEditTarget = "new_comment"
			return m, openExternalEditorCmd(m.commentInput.Value())
		case "esc":
			m.commentInput.SetValue("")
			m.commentInput.Blur()
			m.state = m.commentReturnState
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	m.refreshMentionSuggestions()
	return m, cmd
}

func (m *AppModel) setCommentValueAndCursor(value string, cursorIndex int) {
	m.commentInput.SetValue(value)
	if cursorIndex < 0 {
		cursorIndex = 0
	}

	runes := []rune(value)
	if cursorIndex > len(runes) {
		cursorIndex = len(runes)
	}

	row := 0
	col := 0
	for i := 0; i < cursorIndex; i++ {
		if runes[i] == '\n' {
			row++
			col = 0
			continue
		}
		col++
	}

	m.commentInput.CursorStart()
	for i := 0; i < row; i++ {
		m.commentInput.CursorDown()
	}
	m.commentInput.SetCursor(col)
}

func (m *AppModel) applyMentionSuggestion(index int) {
	if index < 0 || index >= len(m.mentionSuggestions) {
		return
	}

	member := m.mentionSuggestions[index]
	username := memberUsername(member)
	if username == "" {
		return
	}

	runes := []rune(m.commentInput.Value())
	before := string(runes[:m.mentionQueryStart])
	after := string(runes[m.mentionQueryEnd:])
	inserted := "@" + username + " "
	newValue := before + inserted + after
	m.setCommentValueAndCursor(newValue, len([]rune(before))+len([]rune(inserted)))
	m.refreshMentionSuggestions()
}

func (m *AppModel) handleMentionKey(msg tea.KeyMsg) bool {
	if len(m.mentionSuggestions) == 0 {
		return false
	}

	switch msg.String() {
	case "up", "ctrl+p", "shift+tab":
		if m.mentionSelectedIdx > 0 {
			m.mentionSelectedIdx--
		} else {
			m.mentionSelectedIdx = len(m.mentionSuggestions) - 1
		}
		return true
	case "down", "ctrl+n", "tab":
		m.mentionSelectedIdx = (m.mentionSelectedIdx + 1) % len(m.mentionSuggestions)
		return true
	case "enter":
		m.applyMentionSuggestion(m.mentionSelectedIdx)
		return true
	}

	return false
}

func (m *AppModel) updateEditComment(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.handleMentionKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+s":
			v := m.commentInput.Value()
			if v != "" {
				m.commentInput.SetValue("")
				m.commentInput.Blur()
				m.state = m.commentReturnState
				m.popupMsg = "Updating comment..."
				return m, tea.Batch(m.spinner.Tick, editCommentCmd(m.client, m.editingCommentID, v, m.mentionableMembers()))
			}
			return m, nil
		case "ctrl+e":
			m.externalEditTarget = "comment"
			return m, openExternalEditorCmd(m.commentInput.Value())
		case "esc":
			m.commentInput.SetValue("")
			m.commentInput.Blur()
			m.state = m.commentReturnState
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	m.refreshMentionSuggestions()
	return m, cmd
}

func (m *AppModel) updateCreateTask(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.taskInput.Value())
			if name != "" {
				m.taskInput.Blur()
				m.state = stateTasks
				m.loading = true
				m.loadingMsg = "Creating task..."
				return m, tea.Batch(m.spinner.Tick, createTaskCmd(m.client, m.selectedTeam, m.selectedList, name, m.currentUserID))
			}
		case tea.KeyEsc:
			m.taskInput.Blur()
			m.state = stateTasks
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)
	return m, cmd
}

func (m *AppModel) updateCreateSubtask(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.taskInput.Value())
			if name != "" {
				m.taskInput.Blur()
				m.state = stateTaskDetail
				m.loading = true
				m.loadingMsg = "Creating subtask..."
				return m, tea.Batch(m.spinner.Tick, createSubtaskCmd(m.client, m.selectedTeam, m.selectedList, m.parentTaskID, name, m.currentUserID))
			}
		case tea.KeyEsc:
			m.taskInput.Blur()
			m.state = stateTaskDetail
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)
	return m, cmd
}

func (m *AppModel) updateMovePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.state = stateTaskDetail
			m.updateViewportContent()
			return m, nil
		case "up", "k":
			if m.suggestIdx > 0 {
				m.suggestIdx--
			}
			return m, nil
		case "down", "j":
			if m.suggestIdx < len(m.moveCandidateLists)-1 {
				m.suggestIdx++
			}
			return m, nil
		case "enter":
			if m.suggestIdx < len(m.moveCandidateLists) {
				destList := m.moveCandidateLists[m.suggestIdx]
				m.state = stateTaskDetail
				m.loading = true
				m.loadingMsg = "Moving task to " + destList.Name + "..."
				taskID := m.moveTaskID
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					return errMsg(m.client.MoveTask(taskID, destList.ID))
				})
			}
		}
	}
	return m, nil
}

func (m *AppModel) updateEditTitle(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.taskInput.Blur()
			m.state = stateTaskDetail
			return m, nil
		case tea.KeyEnter:
			name := strings.TrimSpace(m.taskInput.Value())
			if name == "" {
				return m, nil
			}
			m.taskInput.Blur()
			m.state = stateTaskDetail
			if err := m.client.UpdateTaskName(m.selectedTask.ID, name); err == nil {
				m.selectedTask.Name = name
				for i, t := range m.allTasks {
					if t.ID == m.selectedTask.ID {
						m.allTasks[i].Name = name
						break
					}
				}
				m.updateViewportContent()
				m.applyTaskFilter("")
				m.popupMsg = "Updated title"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
		}
	}
	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)
	return m, cmd
}

func (m *AppModel) updateEditDesc(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.descInput.Blur()
			if m.hasUnsavedDescriptionChanges() {
				m.state = stateConfirmDiscardDesc
				return m, nil
			}
			m.state = stateTaskDetail
			return m, nil
		case tea.KeyCtrlS:
			desc := m.descInput.Value()
			m.descInput.Blur()
			m.state = stateTaskDetail
			m.loading = true
			m.loadingMsg = "Updating description..."
			return m, tea.Batch(m.spinner.Tick, updateDescriptionCmd(m.client, m.selectedTask.ID, m.selectedTeam, desc, m.detailBackState))
		}
	}
	var cmd tea.Cmd
	m.descInput, cmd = m.descInput.Update(msg)
	return m, cmd
}

func (m *AppModel) applyTaskDetail(task *clickup.Task, comments []clickup.Comment, backState state, preserveHistory bool) (tea.Model, tea.Cmd) {
	m.loading = false
	if task == nil {
		return m, nil
	}

	previousTaskID := m.selectedTask.ID
	wasChecklist := m.state == stateChecklist || m.state == stateConfirmChecklistDelete

	m.selectedTask = *task
	m.detailBackState = backState
	m.selectedComments = comments

	if !preserveHistory && !wasChecklist {
		m.taskHistory = nil
	}

	if wasChecklist {
		m.flattenChecklists()
		m.updateChecklistViewportContent()
	} else {
		m.state = stateTaskDetail
		m.prevState = backState
		m.updateViewportContent()
		if previousTaskID != m.selectedTask.ID {
			m.vp.GotoTop()
		}
	}

	if m.selectedTask.ID != "" && m.teamMembersTaskID != m.selectedTask.ID {
		return m, fetchTaskMembersCmd(m.client, m.selectedTask.ID)
	}
	return m, nil
}

func (m *AppModel) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "left", "enter":
			m.state = m.prevState
			if m.state == stateTaskDetail {
				m.updateViewportContent()
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateConfirmProfileDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			name := m.pendingDeleteProfile
			wasActive := m.cfg.ActiveProfileName() == name
			nextProfile, ok := m.cfg.DeleteProfile(name)
			m.pendingDeleteProfile = ""
			if !ok {
				m.state = m.prevState
				m.popupMsg = "Error: failed to delete profile"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			if err := config.SaveConfig(m.cfg); err != nil {
				m.state = m.prevState
				m.popupMsg = "Error: failed to save profile deletion"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			if wasActive {
				m.loading = true
				m.loadingMsg = "Switching to profile " + nextProfile + "..."
				return m, tea.Batch(m.spinner.Tick, reloadProfileCmd(m.cfg, m.width, m.height, "Deleted profile "+name))
			}
			m.state = m.prevState
			m.popupMsg = "Deleted profile " + name
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		case "n", "esc", "q":
			m.pendingDeleteProfile = ""
			m.state = m.prevState
			m.popupMsg = "Profile deletion cancelled"
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
	}
	return m, nil
}

func (m *AppModel) updateConfirmListDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			listID := m.pendingDeleteListID
			listName := m.pendingDeleteListName
			folderID := m.pendingDeleteListFolderID
			m.pendingDeleteListID = ""
			m.pendingDeleteListName = ""
			m.pendingDeleteListFolderID = ""
			if m.selectedSpace == "" {
				m.state = m.prevState
				m.popupMsg = "Error: select a Space first"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			m.loading = true
			m.loadingMsg = "Deleting list..."
			return m, tea.Batch(m.spinner.Tick, deleteListCmd(m.client, m.selectedSpace, listID, folderID, listName))
		case "n", "esc", "q":
			m.pendingDeleteListID = ""
			m.pendingDeleteListName = ""
			m.pendingDeleteListFolderID = ""
			m.state = m.prevState
			m.popupMsg = "List deletion cancelled"
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
	}
	return m, nil
}

func (m *AppModel) updateConfirmTaskDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			taskID := m.pendingDeleteTaskID
			taskName := m.pendingDeleteTaskName
			m.pendingDeleteTaskID = ""
			m.pendingDeleteTaskName = ""
			if m.selectedList == "" {
				m.state = m.prevState
				m.popupMsg = "Error: select a List first"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			m.loading = true
			m.loadingMsg = "Deleting task..."
			return m, tea.Batch(m.spinner.Tick, deleteTaskCmd(m.client, m.selectedTeam, m.selectedList, taskID, taskName))
		case "n", "esc", "q":
			m.pendingDeleteTaskID = ""
			m.pendingDeleteTaskName = ""
			m.state = m.prevState
			m.popupMsg = "Task deletion cancelled"
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
	}
	return m, nil
}

func (m *AppModel) updateConfirmSpaceDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			spaceID := m.pendingDeleteSpaceID
			spaceName := m.pendingDeleteSpaceName
			m.pendingDeleteSpaceID = ""
			m.pendingDeleteSpaceName = ""

			teamID := m.selectedTeam
			if teamID == "" && m.cfg != nil {
				teamID = m.cfg.ClickupTeamID
			}
			if teamID == "" {
				m.state = m.prevState
				m.popupMsg = "Error: select a Workspace first"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			m.loading = true
			m.loadingMsg = "Deleting space..."
			return m, tea.Batch(m.spinner.Tick, deleteSpaceCmd(m.client, teamID, spaceID, spaceName))
		case "n", "esc", "q":
			m.pendingDeleteSpaceID = ""
			m.pendingDeleteSpaceName = ""
			m.state = m.prevState
			m.popupMsg = "Space deletion cancelled"
			return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
		}
	}
	return m, nil
}

func (m *AppModel) updateConfirmDiscardDesc(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			m.descInput.SetValue(m.editableDescription())
			m.state = stateTaskDetail
			return m, nil
		case "n", "esc", "q":
			m.state = stateEditDesc
			m.descInput.Focus()
			return m, textarea.Blink
		}
	}
	return m, nil
}

func (m *AppModel) updateChecklist(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()

		if m.isEditingChecklistItem() {
			switch msg.Type {
			case tea.KeyEnter:
				newValue := strings.TrimSpace(m.checklistEditInput.Value())
				original := m.getChecklistEditOriginal()
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Blur()
				editingItem := m.checklistEditingItem
				m.checklistEditingItem = nil

				if newValue != "" {

					if editingItem == nil {
						m.loading = true
						m.loadingMsg = "Creating checklist..."
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.CreateChecklist(m.selectedTask.ID, newValue); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					}

					if m.checklistEditOriginal == "" {

						m.loading = true
						m.loadingMsg = "Adding item..."
						checklist := editingItem.checklist
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.CreateChecklistItem(checklist.ID, newValue); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					} else if newValue != original {

						m.loading = true
						m.loadingMsg = "Updating..."
						if m.checklistEditOriginal == editingItem.checklist.Name {
							checklist := editingItem.checklist
							return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
								if err := m.client.UpdateChecklist(checklist.ID, newValue); err != nil {
									return errMsg(err)
								}
								return checklistItemUpdatedMsg{}
							})
						} else {
							item := editingItem.item
							checklist := editingItem.checklist
							return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
								if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, newValue, item.Resolved, nil); err != nil {
									return errMsg(err)
								}
								return checklistItemUpdatedMsg{}
							})
						}
					}
				}
				return m, nil

			case tea.KeyEsc:
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Blur()
				m.checklistEditingItem = nil
				m.updateChecklistViewportContent()
				return m, nil
			}

			var cmd tea.Cmd
			m.checklistEditInput, cmd = m.checklistEditInput.Update(msg)
			m.updateChecklistViewportContent()
			return m, cmd
		}

		switch s {
		case "q":
			m.checklistViewItems = nil
			m.checklistSelectedIdx = 0
			m.state = stateTaskDetail
			m.updateViewportContent()
			return m, nil

		case "up", "k":
			if m.checklistSelectedIdx > 0 {
				m.checklistSelectedIdx--
			}
			m.updateChecklistViewportContent()
			return m, nil

		case "down", "j":
			if m.checklistSelectedIdx < len(m.checklistViewItems)-1 {
				m.checklistSelectedIdx++
			}
			m.updateChecklistViewportContent()
			return m, nil

		case "tab":
			if m.checklistSelectedIdx > 0 && m.checklistSelectedIdx < len(m.checklistViewItems) {
				currentItem := m.checklistViewItems[m.checklistSelectedIdx]
				if currentItem.itemType == checklistTypeItem {
					// Find the nearest preceding item with the same parent
					var targetParentID string
					foundTarget := false

					for i := m.checklistSelectedIdx - 1; i >= 0; i-- {
						prevItem := m.checklistViewItems[i]
						if prevItem.itemType == checklistTypeItem && prevItem.checklist.ID == currentItem.checklist.ID {

							sameParent := false
							if currentItem.item.Parent == nil || *currentItem.item.Parent == "" {
								sameParent = (prevItem.item.Parent == nil || *prevItem.item.Parent == "")
							} else {
								sameParent = (prevItem.item.Parent != nil && *prevItem.item.Parent == *currentItem.item.Parent)
							}

							if sameParent {
								targetParentID = prevItem.item.ID
								foundTarget = true
								break
							}
						}
					}

					if foundTarget {
						m.loading = true
						m.loadingMsg = "Indenting..."
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.UpdateChecklistItem(currentItem.checklist.ID, currentItem.item.ID, currentItem.item.Name, currentItem.item.Resolved, &targetParentID); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					}
				}
			}
			return m, nil

		case "shift+tab":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				currentItem := m.checklistViewItems[m.checklistSelectedIdx]
				if currentItem.itemType == checklistTypeItem {
					if currentItem.item.Parent != nil && *currentItem.item.Parent != "" {
						var grandparentID string
						for i := m.checklistSelectedIdx - 1; i >= 0; i-- {
							prevItem := m.checklistViewItems[i]
							if prevItem.itemType == checklistTypeItem && prevItem.checklist.ID == currentItem.checklist.ID {
								if prevItem.item.ID == *currentItem.item.Parent {
									if prevItem.item.Parent != nil && *prevItem.item.Parent != "" {
										grandparentID = *prevItem.item.Parent
									} else {
										grandparentID = ""
									}
									break
								}
							}
						}

						m.loading = true
						m.loadingMsg = "Outdenting..."
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.UpdateChecklistItem(currentItem.checklist.ID, currentItem.item.ID, currentItem.item.Name, currentItem.item.Resolved, &grandparentID); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					}
				}
			}
			return m, nil

		case " ":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeItem {
					m.loading = true
					m.loadingMsg = "Toggling..."
					checklist := item.checklist
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.UpdateChecklistItem(checklist.ID, item.item.ID, item.item.Name, !item.item.Resolved, nil); err != nil {
							return errMsg(err)
						}
						return checklistItemUpdatedMsg{}
					})
				}
			}
			return m, nil

		case "enter", "e":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditOriginal = m.getChecklistEditOriginal()
				m.checklistEditInput.SetValue(m.checklistEditOriginal)
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
				m.updateChecklistViewportContent()
				return m, textinput.Blink
			}
			return m, nil

		case "a":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditOriginal = ""
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Placeholder = "New item name..."
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(0)
				m.updateChecklistViewportContent()
				return m, textinput.Blink
			}
			return m, nil

		case "r":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditOriginal = m.getChecklistEditOriginal()
				m.checklistEditInput.SetValue(m.checklistEditOriginal)
				m.checklistEditInput.Placeholder = "Name..."
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
				m.updateChecklistViewportContent()
				return m, textinput.Blink
			}
			return m, nil

		case "d":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeItem {
					m.loading = true
					m.loadingMsg = "Deleting item..."
					checklist := item.checklist
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.DeleteChecklistItem(checklist.ID, item.item.ID); err != nil {
							return errMsg(err)
						}
						return checklistItemUpdatedMsg{}
					})
				} else {
					m.checklistPendingDelete = item.checklist
					m.state = stateConfirmChecklistDelete
					return m, nil
				}
			}
			return m, nil

		case "R", "D":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeHeader {
					if s == "D" {
						m.checklistPendingDelete = item.checklist
						m.state = stateConfirmChecklistDelete
						return m, nil
					}
					m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
					m.checklistEditOriginal = m.getChecklistEditOriginal()
					m.checklistEditInput.SetValue(item.checklist.Name)
					m.checklistEditInput.Placeholder = "Checklist name..."
					m.checklistEditInput.Focus()
					m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
					return m, textinput.Blink
				}
			}
			return m, nil

		case "n":
			m.checklistEditInput.SetValue("")
			m.checklistEditInput.Placeholder = "New checklist name..."
			m.checklistEditInput.Focus()
			m.checklistEditInput.SetCursor(0)
			m.checklistEditingItem = nil
			m.checklistPendingDelete = clickup.Checklist{}
			m.updateChecklistViewportContent()
			return m, textinput.Blink
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateCommentsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.commentSelectedIdx > 0 {
				m.commentSelectedIdx--
			}
			m.updateCommentsViewportContent()
		case "down", "j":
			if m.commentSelectedIdx < len(m.selectedComments)-1 {
				m.commentSelectedIdx++
			}
			m.updateCommentsViewportContent()
		case "c":
			m.commentReturnState = m.state
			m.state = stateComment
			m.commentInput.SetValue("")
			m.commentInput.Focus()
			m.refreshMentionSuggestions()
			return m, textinput.Blink
		case "r":
			if len(m.selectedComments) > 0 && m.commentSelectedIdx >= 0 {
				c := m.selectedComments[m.commentSelectedIdx]
				m.replyToCommentID = c.ID
				m.replyToUser = c.User.Username
				m.commentReturnState = m.state
				m.state = stateComment
				m.commentInput.SetValue("")
				m.commentInput.Focus()
				m.refreshMentionSuggestions()
				return m, textinput.Blink
			}
		case "e":
			if len(m.selectedComments) > 0 && m.commentSelectedIdx >= 0 {
				c := m.selectedComments[m.commentSelectedIdx]
				m.editingCommentID = c.ID
				m.commentInput.SetValue(c.CommentText)
				m.commentInput.Focus()
				m.commentReturnState = m.state
				m.state = stateEditComment
				m.refreshMentionSuggestions()
				return m, textinput.Blink
			}
		case "d":
			if len(m.selectedComments) > 0 && m.commentSelectedIdx >= 0 {
				if m.selectedComments[m.commentSelectedIdx].Parent != nil && *m.selectedComments[m.commentSelectedIdx].Parent != "" {
					m.popupMsg = "Thread replies cannot be deleted via the ClickUp API"
					return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.state = stateConfirmCommentDelete
				return m, nil
			}
		case "esc", "q", "left":
			m.state = stateTaskDetail
			m.updateViewportContent()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateConfirmCommentDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			if m.commentSelectedIdx >= 0 && m.commentSelectedIdx < len(m.selectedComments) {
				c := m.selectedComments[m.commentSelectedIdx]
				if c.Parent != nil && *c.Parent != "" {
					m.popupMsg = "Thread replies cannot be deleted via the ClickUp API"
					m.state = stateCommentsView
					return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Deleting comment..."
				m.state = stateCommentsView
				return m, tea.Batch(m.spinner.Tick, deleteCommentCmd(m.client, c.ID))
			}
		case "n", "esc", "q":
			m.state = stateCommentsView
			return m, nil
		}
	}
	return m, nil
}

func (m *AppModel) updateConfirmChecklistDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			checklist := m.checklistPendingDelete
			m.checklistPendingDelete = clickup.Checklist{}
			m.loading = true
			m.loadingMsg = "Deleting checklist..."
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				if err := m.client.DeleteChecklist(checklist.ID); err != nil {
					return errMsg(err)
				}
				return checklistDeletedMsg{}
			})
		case "n", "esc", "q":
			m.checklistPendingDelete = clickup.Checklist{}
			m.state = stateChecklist
			return m, nil
		}
	}
	return m, nil
}

func (m *AppModel) updateCommand(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyDown:
			if len(m.filteredSuggest) > 0 {
				m.suggestIdx++
				if m.suggestIdx >= len(m.filteredSuggest) {
					m.suggestIdx = 0
				}
			}
			return m, nil
		case tea.KeyUp, tea.KeyShiftTab:
			if len(m.filteredSuggest) > 0 {
				m.suggestIdx--
				if m.suggestIdx < 0 {
					m.suggestIdx = len(m.filteredSuggest) - 1
				}
			}
			return m, nil
		case tea.KeyTab:
			if len(m.filteredSuggest) > 0 {

				m.cmdInput.SetValue(m.filteredSuggest[m.suggestIdx].Text)
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.filterSuggestions()
			}
			return m, nil
		case tea.KeyEnter:
			val := m.cmdInput.Value()

			if len(m.filteredSuggest) > 0 && m.suggestIdx >= 0 {
				val = m.filteredSuggest[m.suggestIdx].Text
			}

			m.cmdInput.SetValue("")
			m.cmdInput.Blur()
			m.state = m.prevState
			m.filterSuggestions()
			m.updateLayout()

			if strings.HasPrefix(val, "/filter ") {
				term := strings.TrimPrefix(val, "/filter ")
				m.applyHierarchyFilter(term)
			} else if strings.HasPrefix(val, "/clear") {
				m.applyHierarchyFilter("")
			} else if strings.HasPrefix(val, "/help") {
				m.state = stateHelp
				m.updateHelpContent()
			} else if strings.HasPrefix(val, "/search ") {
				query := strings.TrimSpace(strings.TrimPrefix(val, "/search "))
				if query != "" {
					teamID := m.selectedTeam
					if teamID == "" && m.cfg != nil {
						teamID = m.cfg.ClickupTeamID
					}
					if teamID == "" && len(m.allTeams) > 0 {
						teamID = m.allTeams[0].ID
					}
					m.loading = true
					m.loadingMsg = "Searching tickets..."
					return m, tea.Batch(m.spinner.Tick, searchTasksCmd(m.client, teamID, query))
				}
			} else if strings.HasPrefix(val, "/space create ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/space create "))
				if name == "" {
					m.popupMsg = "Error: space name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				teamID := m.selectedTeam
				if teamID == "" && m.cfg != nil {
					teamID = m.cfg.ClickupTeamID
				}
				if teamID == "" {
					m.popupMsg = "Error: select a Workspace first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Creating space..."
				return m, tea.Batch(m.spinner.Tick, createSpaceCmd(m.client, teamID, name))
			} else if strings.HasPrefix(val, "/space rename ") {
				if m.prevState != stateSpaces {
					m.popupMsg = "Error: /space rename only works from the spaces view"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				name := strings.TrimSpace(strings.TrimPrefix(val, "/space rename "))
				if name == "" {
					m.popupMsg = "Error: new space name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				selected, ok := m.activeList.SelectedItem().(spaceItem)
				if !ok {
					m.popupMsg = "Error: highlight a space first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				teamID := m.selectedTeam
				if teamID == "" && m.cfg != nil {
					teamID = m.cfg.ClickupTeamID
				}
				if teamID == "" {
					m.popupMsg = "Error: select a Workspace first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Renaming space..."
				return m, tea.Batch(m.spinner.Tick, renameSpaceCmd(m.client, teamID, selected.ID, name))
			} else if strings.HasPrefix(val, "/space delete") {
				if m.prevState != stateSpaces {
					m.popupMsg = "Error: /space delete only works from the spaces view"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				selected, ok := m.activeList.SelectedItem().(spaceItem)
				if !ok {
					m.popupMsg = "Error: highlight a space first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteSpaceID = selected.ID
				m.pendingDeleteSpaceName = selected.Name
				m.prevState = m.state
				m.state = stateConfirmSpaceDelete
				return m, nil
			} else if strings.HasPrefix(val, "/list create ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/list create "))
				if name == "" {
					m.popupMsg = "Error: list name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if m.selectedSpace == "" {
					m.popupMsg = "Error: select a Space first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				folderID := ""
				if m.selectedFolder != nil {
					folderID = m.selectedFolder.ID
				}
				m.loading = true
				if folderID != "" {
					m.loadingMsg = "Creating list in folder..."
				} else {
					m.loadingMsg = "Creating list in space..."
				}
				return m, tea.Batch(m.spinner.Tick, createListCmd(m.client, m.selectedSpace, folderID, name))
			} else if strings.HasPrefix(val, "/list rename ") {
				if m.prevState != stateLists {
					m.popupMsg = "Error: /list rename only works from the list view"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				name := strings.TrimSpace(strings.TrimPrefix(val, "/list rename "))
				if name == "" {
					m.popupMsg = "Error: new list name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				selected, ok := m.activeList.SelectedItem().(listItem)
				if !ok {
					m.popupMsg = "Error: highlight a list first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if m.selectedSpace == "" {
					m.popupMsg = "Error: select a Space first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				folderID := ""
				if m.selectedFolder != nil {
					folderID = m.selectedFolder.ID
				}
				m.loading = true
				m.loadingMsg = "Renaming list..."
				return m, tea.Batch(m.spinner.Tick, renameListCmd(m.client, m.selectedSpace, selected.ID, folderID, name))
			} else if strings.HasPrefix(val, "/list delete") {
				if m.prevState != stateLists {
					m.popupMsg = "Error: /list delete only works from the list view"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				selected, ok := m.activeList.SelectedItem().(listItem)
				if !ok {
					m.popupMsg = "Error: highlight a list first"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteListID = selected.ID
				m.pendingDeleteListName = selected.Name
				m.pendingDeleteListFolderID = ""
				if m.selectedFolder != nil {
					m.pendingDeleteListFolderID = m.selectedFolder.ID
				}
				m.state = stateConfirmListDelete
				return m, nil
			} else if strings.HasPrefix(val, "/profile create ") {
				name, token, err := parseProfileCreateInput(strings.TrimSpace(strings.TrimPrefix(val, "/profile create ")))
				if err != nil {
					m.popupMsg = "Error: invalid /profile create syntax"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if name == "" {
					m.popupMsg = "Error: profile name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if !m.cfg.CreateProfile(name) {
					m.popupMsg = "Error: profile already exists"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if err := config.SaveConfig(m.cfg); err != nil {
					m.popupMsg = "Error: failed to create profile"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if token != "" {
					m.cfg.ClickupAPIKey = token
					if err := config.SaveConfig(m.cfg); err != nil {
						m.popupMsg = "Error: failed to save API key"
						return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
					}
					m.loading = true
					m.loadingMsg = "Loading profile " + name + "..."
					return m, tea.Batch(m.spinner.Tick, reloadProfileCmd(m.cfg, m.width, m.height, "Created and switched to profile "+name))
				}
				reloaded := newBaseModel(m.cfg)
				reloaded.width = m.width
				reloaded.height = m.height
				reloaded.updateLayout()
				reloaded.popupMsg = "Created and switched to profile " + name
				*m = *reloaded
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			} else if strings.HasPrefix(val, "/profile switch ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/profile switch "))
				if name == "" {
					m.popupMsg = "Error: profile name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if !m.cfg.SetActiveProfile(name) {
					m.popupMsg = "Error: profile not found"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if err := config.SaveConfig(m.cfg); err != nil {
					m.popupMsg = "Error: failed to save profile switch"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Switching to profile " + name + "..."
				return m, tea.Batch(m.spinner.Tick, reloadProfileCmd(m.cfg, m.width, m.height, "Switched profile to "+name))
			} else if strings.HasPrefix(val, "/profile delete ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/profile delete "))
				if name == "" {
					m.popupMsg = "Error: profile name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				if !m.cfg.HasProfile(name) {
					m.popupMsg = "Error: profile not found"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.pendingDeleteProfile = name
				m.state = stateConfirmProfileDelete
				return m, nil
			} else if strings.HasPrefix(val, "/profile save ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/profile save "))
				if name == "" {
					m.popupMsg = "Error: profile name required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.cfg.SaveCurrentAsProfile(name)
				if err := config.SaveConfig(m.cfg); err != nil {
					m.popupMsg = "Error: failed to save profile"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Switching to profile " + name + "..."
				return m, tea.Batch(m.spinner.Tick, reloadProfileCmd(m.cfg, m.width, m.height, "Saved and switched to profile "+name))
			} else if strings.HasPrefix(val, "/profile token ") {
				token := strings.TrimSpace(strings.TrimPrefix(val, "/profile token "))
				if token == "" {
					m.popupMsg = "Error: API key required"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.cfg.ClickupAPIKey = token
				if err := config.SaveConfig(m.cfg); err != nil {
					m.popupMsg = "Error: failed to save API key"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
				m.loading = true
				m.loadingMsg = "Reloading profile " + m.cfg.ActiveProfileName() + "..."
				return m, tea.Batch(m.spinner.Tick, reloadProfileCmd(m.cfg, m.width, m.height, "Updated API key for profile "+m.cfg.ActiveProfileName()))
			} else if strings.HasPrefix(val, "/status ") {
				if m.prevState == stateTaskDetail {
					newStatus := strings.TrimPrefix(val, "/status ")
					if strings.TrimSpace(newStatus) == "" {
						m.popupMsg = "Error: status required"
						return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
					}
					m.loading = true
					m.loadingMsg = "Updating status..."
					return m, tea.Batch(m.spinner.Tick, updateStatusCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.selectedList, newStatus))
				}
			} else if strings.HasPrefix(val, "/priority ") {
				if m.prevState == stateTaskDetail {
					input := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(val, "/priority ")))
					var p *int
					valid := true
					switch input {
					case "urgent", "1":
						v := 1
						p = &v
					case "high", "2":
						v := 2
						p = &v
					case "normal", "3":
						v := 3
						p = &v
					case "low", "4":
						v := 4
						p = &v
					case "none", "no", "clear", "0":
						p = nil
					default:
						valid = false
					}
					if valid {
						m.loading = true
						m.loadingMsg = "Updating priority..."
						return m, tea.Batch(m.spinner.Tick, setPriorityCmd(m.client, m.selectedTask.ID, m.selectedTeam, p))
					} else {
						m.popupMsg = "Error: Invalid priority. Use urgent, high, normal, low, or none."
						return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
					}
				}
			} else if strings.HasPrefix(val, "/ticket ") {
				id := strings.TrimSpace(strings.TrimPrefix(val, "/ticket "))
				if id != "" {
					teamID := m.selectedTeam
					if teamID == "" && len(m.allTeams) > 0 {
						teamID = m.allTeams[0].ID
					}
					m.loading = true
					m.loadingMsg = "Loading ticket " + id + "..."
					return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, id, teamID, m.prevState))
				}
			} else if strings.HasPrefix(val, "/points ") {
				if m.prevState == stateTaskDetail {
					ptStr := strings.TrimPrefix(val, "/points ")
					pts, err := strconv.ParseFloat(ptStr, 64)
					if err == nil {
						m.client.UpdatePoints(m.selectedTask.ID, pts)
						m.selectedTask.Points = &pts
						m.updateViewportContent()
						for i, t := range m.allTasks {
							if t.ID == m.selectedTask.ID {
								m.allTasks[i].Points = &pts
								break
							}
						}
						m.applyHierarchyFilter(strings.TrimPrefix(m.cmdInput.Value(), "/filter "))
					}
				}
			} else if strings.HasPrefix(val, "/share") {
				if m.prevState == stateTaskDetail && m.selectedTask.URL != "" {
					clipboard.WriteAll(m.selectedTask.URL)
					m.popupMsg = "Copied URL to Clipboard"
					return m, tea.Tick(time.Second*1, func(_ time.Time) tea.Msg {
						return clearPopupMsg{}
					})
				}
			} else if strings.HasPrefix(val, "/delete") {
				if m.prevState == stateTaskDetail {
					taskID := m.selectedTask.ID
					m.loading = true
					m.loadingMsg = "Deleting task..."
					m.state = stateTasks
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.DeleteTask(taskID); err != nil {
							return errMsg(err)
						}
						// Remove from local cache
						var kept []clickup.Task
						for _, t := range m.allTasks {
							if t.ID != taskID {
								kept = append(kept, t)
							}
						}
						m.allTasks = kept
						m.applyTaskFilter("")
						return tasksMsg(m.allTasks)
					})
				}
			} else if strings.HasPrefix(val, "/move") {
				if m.prevState == stateTaskDetail {
					m.moveTaskID = m.selectedTask.ID
					m.suggestIdx = 0
					m.loading = true
					m.loadingMsg = "Loading available lists..."
					return m, tea.Batch(m.spinner.Tick, fetchAllListsForMoveCmd(m.client, m.selectedSpace))
				}
			} else if strings.HasPrefix(val, "/edit ") {
				if m.prevState == stateTaskDetail {
					target := strings.ToLower(strings.TrimPrefix(val, "/edit "))
					switch target {
					case "title":
						m.state = stateEditTitle
						m.taskInput.SetValue(m.selectedTask.Name)
						m.taskInput.Focus()
						m.taskInput.SetCursor(len(m.taskInput.Value()))
						return m, textinput.Blink
					case "desc":
						m.state = stateEditDesc
						m.descInput.SetValue(m.editableDescription())
						m.refreshEditDescLayout()
						m.descInput.Focus()
						return m, textarea.Blink
					case "desc externally":
						m.externalEditTarget = "description"
						return m, openExternalEditorCmd(m.editableDescription())
					}
					m.popupMsg = "Unknown /edit command. Use /edit [title|desc|desc externally]"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
			} else if strings.HasPrefix(val, "/copy ") {
				if m.prevState == stateTaskDetail {
					parts := strings.Fields(val)
					if len(parts) >= 2 {
						target := strings.ToLower(parts[1])
						switch target {
						case "title":
							return m, m.copyTaskTitle()
						case "desc":
							return m, m.copyTaskDescription()
						case "checklist":
							return m, m.copyTaskChecklists()
						case "all":
							return m, m.copyTaskForAI()
						}
					}
					m.popupMsg = "Unknown /copy command. Use /copy [title|desc|checklist|all]"
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
			} else if strings.HasPrefix(val, "/subtask") {
				if m.prevState == stateTaskDetail {
					m.parentTaskID = m.selectedTask.ID
					m.state = stateCreateSubtask
					m.taskInput.SetValue("")
					m.taskInput.Focus()
					return m, textinput.Blink
				}
			} else if strings.HasPrefix(val, "/checklist ") {
				if m.prevState == stateTaskDetail {
					parts := strings.Fields(val)
					if len(parts) >= 3 {
						action := strings.ToLower(parts[1])
						switch action {
						case "add":
							name := strings.TrimSpace(strings.TrimPrefix(val, "/checklist add "))
							if name == "" {
								m.popupMsg = "Error: checklist name required"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							m.loading = true
							m.loadingMsg = "Creating checklist..."
							return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
								if err := m.client.CreateChecklist(m.selectedTask.ID, name); err != nil {
									return errMsg(err)
								}
								return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
							})
						case "rename":
							if len(parts) < 4 {
								m.popupMsg = "Error: checklist number and new name required"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							idx, err := strconv.Atoi(parts[2])
							if err != nil || idx < 1 || idx > len(m.selectedTask.Checklists) {
								m.popupMsg = "Error: invalid checklist number"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							name := strings.TrimSpace(strings.SplitN(val, parts[2], 2)[1])
							name = strings.TrimSpace(strings.TrimPrefix(name, parts[2]))
							if name == "" {
								name = strings.TrimSpace(strings.TrimPrefix(val, "/checklist rename "+parts[2]))
							}
							if name == "" {
								m.popupMsg = "Error: new checklist name required"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							checklist := m.selectedTask.Checklists[idx-1]
							m.loading = true
							m.loadingMsg = "Renaming checklist..."
							return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
								if err := m.client.UpdateChecklist(checklist.ID, name); err != nil {
									return errMsg(err)
								}
								return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
							})
						case "delete":
							idx, err := strconv.Atoi(parts[2])
							if err != nil || idx < 1 || idx > len(m.selectedTask.Checklists) {
								m.popupMsg = "Error: invalid checklist number"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							checklist := m.selectedTask.Checklists[idx-1]
							m.loading = true
							m.loadingMsg = "Deleting checklist..."
							return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
								if err := m.client.DeleteChecklist(checklist.ID); err != nil {
									return errMsg(err)
								}
								return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
							})
						case "item":
							if len(parts) < 5 {
								m.popupMsg = "Error: checklist item command is incomplete"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							itemAction := strings.ToLower(parts[2])
							checklistIdx, err := strconv.Atoi(parts[3])
							if err != nil || checklistIdx < 1 || checklistIdx > len(m.selectedTask.Checklists) {
								m.popupMsg = "Error: invalid checklist number"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							checklist := m.selectedTask.Checklists[checklistIdx-1]
							switch itemAction {
							case "add":
								name := strings.TrimSpace(strings.TrimPrefix(val, "/checklist item add "+parts[3]))
								if name == "" {
									m.popupMsg = "Error: checklist item name required"
									return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								m.loading = true
								m.loadingMsg = "Creating checklist item..."
								return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
									if err := m.client.CreateChecklistItem(checklist.ID, name); err != nil {
										return errMsg(err)
									}
									return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
								})
							case "rename", "toggle", "delete":
								if len(parts) < 5 {
									m.popupMsg = "Error: checklist item number required"
									return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								itemIdx, err := strconv.Atoi(parts[4])
								if err != nil || itemIdx < 1 || itemIdx > len(checklist.Items) {
									m.popupMsg = "Error: invalid checklist item number"
									return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								item := checklist.Items[itemIdx-1]
								switch itemAction {
								case "rename":
									name := strings.TrimSpace(strings.TrimPrefix(val, "/checklist item rename "+parts[3]+" "+parts[4]))
									if name == "" {
										m.popupMsg = "Error: new checklist item name required"
										return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
									}
									m.loading = true
									m.loadingMsg = "Renaming checklist item..."
									return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
										if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, name, item.Resolved, nil); err != nil {
											return errMsg(err)
										}
										return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
									})
								case "toggle":
									m.loading = true
									m.loadingMsg = "Updating checklist item..."
									return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
										if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, item.Name, !item.Resolved, nil); err != nil {
											return errMsg(err)
										}
										return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
									})
								case "delete":
									m.loading = true
									m.loadingMsg = "Deleting checklist item..."
									return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
										if err := m.client.DeleteChecklistItem(checklist.ID, item.ID); err != nil {
											return errMsg(err)
										}
										return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
									})
								}
							}
						}
					}
				}
			} else if strings.HasPrefix(val, "/attach ") {
				if m.prevState == stateTaskDetail {
					rest := strings.TrimSpace(strings.TrimPrefix(val, "/attach "))
					action, arg, hasArg := rest, "", false
					if idx := strings.Index(rest, " "); idx >= 0 {
						action = strings.TrimSpace(rest[:idx])
						arg = strings.TrimSpace(rest[idx+1:])
						hasArg = arg != ""
					}
					action = strings.ToLower(action)

					if action == "upload" {
						if !hasArg {
							if err := m.openFilePicker(m.filePickerPath); err != nil {
								m.popupMsg = "Error: " + err.Error()
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
							return m, nil
						}
						sourcePath := expandUserPath(arg)
						if _, err := os.Stat(sourcePath); err != nil {
							m.popupMsg = "Error: file not found"
							return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
						}
						m.loading = true
						m.loadingMsg = "Uploading attachment..."
						return m, tea.Batch(m.spinner.Tick, uploadAttachmentCmd(m.client, m.selectedTask.ID, m.selectedTeam, sourcePath, m.prevState, "Uploaded attachment", nil))
					}
					if hasArg {
						idx, err := strconv.Atoi(arg)
						if err == nil && idx > 0 && idx <= len(m.selectedTask.Attachments) {
							attachment := m.selectedTask.Attachments[idx-1]
							url := attachment.URL
							if action == "open" {
								previewURL := attachmentOpenURL(attachment)
								m.popupMsg = "Opening attachment in browser..."
								return m, tea.Batch(
									openAttachmentURLCmd(previewURL),
									tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
								)
							} else if action == "download" || action == "dl" {
								m.popupMsg = "Starting attachment download..."
								return m, tea.Batch(
									openAttachmentURLCmd(url),
									tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
								)
							} else if action == "share" {
								clipboard.WriteAll(attachmentOpenURL(attachment))
								m.popupMsg = "Copied Attachment URL!"
								return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
							}
						}
					}
				}
			} else if strings.HasPrefix(val, "/comment ") {
				if m.prevState == stateTaskDetail {
					parts := strings.Fields(val)
					if len(parts) >= 3 {
						idx, err := strconv.Atoi(parts[2])
						if err == nil && idx > 0 && idx <= len(m.selectedComments) {
							comment := m.selectedComments[idx-1]
							action := strings.ToLower(parts[1])
							if action == "delete" || action == "del" {
								if comment.Parent != nil && *comment.Parent != "" {
									m.popupMsg = "Thread replies cannot be deleted via the ClickUp API"
									return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								m.loading = true
								m.loadingMsg = "Deleting comment..."
								m.state = stateTaskDetail
								return m, tea.Batch(m.spinner.Tick, deleteCommentCmd(m.client, comment.ID))
							} else if action == "edit" {
								m.state = stateEditComment
								m.editingCommentID = comment.ID
								m.commentInput.SetValue(comment.CommentText)
								m.commentInput.Focus()
								m.refreshMentionSuggestions()
								return m, textinput.Blink
							} else if action == "reply" {
								if comment.Parent != nil && *comment.Parent != "" {
									m.popupMsg = "Error: Cannot reply to a reply"
									return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								m.replyToCommentID = comment.ID
								m.replyToUser = comment.User.Username
								m.state = stateComment
								m.commentInput.SetValue("")
								m.commentInput.Focus()
								m.refreshMentionSuggestions()
								return m, nil
							}
						}
					}
				}
			} else if strings.HasPrefix(val, "/assign ") {
				if m.prevState == stateTaskDetail {

					if len(m.teamMembers) == 0 && m.selectedTeam != "" {
						go func() {}()
					}
					username := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(val, "/assign ")))
					// Look up in team members first, then fall back to task cache
					var addID int
					for _, m2 := range m.teamMembers {
						if strings.ToLower(m2.User.Username) == username {
							addID = m2.User.ID
							break
						}
					}
					if addID == 0 {

						for _, t := range m.allTasks {
							for _, a := range t.Assignees {
								if strings.ToLower(a.Username) == username {
									addID = a.ID
								}
							}
						}
					}
					if addID != 0 {
						var removes []int
						for _, a := range m.selectedTask.Assignees {
							if a.ID != addID {
								removes = append(removes, a.ID)
							}
						}
						taskID := m.selectedTask.ID
						newAssignee := clickup.Assignee{ID: addID, Username: username}
						if err := m.client.UpdateAssignees(taskID, []int{addID}, removes); err == nil {
							m.selectedTask.Assignees = []clickup.Assignee{newAssignee}
							for i, t := range m.allTasks {
								if t.ID == taskID {
									m.allTasks[i].Assignees = []clickup.Assignee{newAssignee}
									break
								}
							}
							m.updateViewportContent()
							m.applyTaskFilter("")
						}
					}
				}
			} else if strings.HasPrefix(val, "/default set") {
				switch m.prevState {
				case stateTeams:
					if i, ok := m.activeList.SelectedItem().(teamItem); ok {
						m.cfg.ClickupTeamID = i.ID
						m.cfg.ClickupSpaceID = ""
						m.cfg.ClickupListID = ""
						config.SaveConfig(m.cfg)
					}
				case stateSpaces:
					if i, ok := m.activeList.SelectedItem().(spaceItem); ok {
						m.cfg.ClickupSpaceID = i.ID
						m.cfg.ClickupTeamID = m.selectedTeam
						m.cfg.ClickupListID = ""
						config.SaveConfig(m.cfg)
					}
				case stateLists:
					if i, ok := m.activeList.SelectedItem().(listItem); ok {
						m.cfg.ClickupListID = i.ID
						m.cfg.ClickupFolderID = ""
						m.cfg.ClickupSpaceID = m.selectedSpace
						m.cfg.ClickupTeamID = m.selectedTeam
						config.SaveConfig(m.cfg)
						m.popupMsg = "Default set successfully"
					} else if i, ok := m.activeList.SelectedItem().(folderItem); ok {
						m.cfg.ClickupFolderID = i.ID
						m.cfg.ClickupListID = ""
						m.cfg.ClickupSpaceID = m.selectedSpace
						m.cfg.ClickupTeamID = m.selectedTeam
						config.SaveConfig(m.cfg)
						m.popupMsg = "Default folder set successfully"
					}
				}
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			} else if strings.HasPrefix(val, "/default user clear") {
				m.cfg.ClickupUserName = ""
				config.SaveConfig(m.cfg)
				m.applyHierarchyFilter(strings.TrimPrefix(m.cmdInput.Value(), "/filter "))
				m.popupMsg = "Default user cleared"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			} else if strings.HasPrefix(val, "/default user ") {
				user := strings.TrimSpace(strings.TrimPrefix(val, "/default user "))
				if user != "" && user != "clear" {
					m.cfg.ClickupUserName = user
					config.SaveConfig(m.cfg)
					m.applyHierarchyFilter(strings.TrimPrefix(m.cmdInput.Value(), "/filter "))
					m.popupMsg = fmt.Sprintf("Default user set: %s", user)
					return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
				}
			} else if strings.HasPrefix(val, "/default clear") {
				m.cfg.ClickupTeamID = ""
				m.cfg.ClickupSpaceID = ""
				m.cfg.ClickupFolderID = ""
				m.cfg.ClickupListID = ""
				config.SaveConfig(m.cfg)
				m.popupMsg = "Routing defaults cleared"
				return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			}
			return m, nil
		case tea.KeyEsc:
			m.cmdInput.SetValue("")
			m.cmdInput.Blur()
			m.state = m.prevState
			m.filterSuggestions()
			m.updateLayout()
			return m, nil
		}
	}

	var cmd tea.Cmd
	old := m.cmdInput.Value()
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	if m.cmdInput.Value() != old {
		m.filterSuggestions()
	}
	return m, cmd
}
