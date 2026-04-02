package ui

import (
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

type checklistDeleteConfirmMsg struct {
	Checklist clickup.Checklist
}

type teamsMsg []clickup.Team

type spacesMsg []clickup.Space

type listsMsg *clickup.SpaceHierarchy

type tasksMsg []clickup.Task

type taskDetailMsg struct {
	Task      *clickup.Task
	Comments  []clickup.Comment
	BackState state
}

type commentsMsg []clickup.Comment

type errMsg error

type clearPopupMsg struct{}

type taskCreatedMsg clickup.Task

type moveListsReadyMsg *clickup.SpaceHierarchy

type profileReloadStartMsg struct {
	Cfg    *config.Config
	Width  int
	Height int
	Popup  string
}

type profileReloadUserMsg struct {
	Model *AppModel
	Popup string
	Err   error
}

type profileReloadTeamsMsg struct {
	Model *AppModel
	Popup string
	Err   error
}

type profileReloadMsg struct {
	Model *AppModel
	Popup string
}

type spaceCreatedMsg struct {
	Spaces []clickup.Space
	Name   string
}

type spaceRenamedMsg struct {
	Spaces []clickup.Space
	Name   string
}

type listCreatedMsg struct {
	Hierarchy *clickup.SpaceHierarchy
	Name      string
}

type listRenamedMsg struct {
	Hierarchy *clickup.SpaceHierarchy
	ListID    string
	FolderID  string
	Name      string
}

type listDeletedMsg struct {
	Hierarchy *clickup.SpaceHierarchy
	FolderID  string
	Name      string
}

type attachmentUploadedMsg struct {
	Task      *clickup.Task
	Comments  []clickup.Comment
	BackState state
	Popup     string
}

type statusUpdatedMsg struct {
	Task     *clickup.Task
	Tasks    []clickup.Task
	Comments []clickup.Comment
}

type teamMembersMsg []clickup.Member

type commentAddedMsg struct{}

type checklistItemUpdatedMsg struct{}

type checklistCreatedMsg struct{}

type checklistDeletedMsg struct{}

type editorFinishedMsg struct {
	content string
	err     error
}

type searchResultsMsg struct {
	Query string
	Tasks []clickup.Task
}
