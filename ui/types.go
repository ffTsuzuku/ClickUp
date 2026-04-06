package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

type state int

const (
	stateTeams state = iota
	stateSpaces
	stateLists
	stateTasks
	stateSearchResults
	stateTaskDetail
	stateComment
	stateCommand
	stateHelp
	stateCreateTask
	stateMovePicker
	stateEditTitle
	stateEditDesc
	stateCreateSubtask
	stateEditComment
	stateConfirmTaskDelete
	stateConfirmProfileDelete
	stateConfirmListDelete
	stateConfirmDiscardDesc
	stateFilePicker
	stateChecklist
	stateConfirmChecklistDelete
	stateConfirmSpaceDelete
	stateCommentsView
	stateConfirmCommentDelete
)

type checklistItemType int

const (
	checklistTypeHeader checklistItemType = iota
	checklistTypeItem
)

type Suggestion struct {
	Text string
	Desc string
}

type AppModel struct {
	state     state
	prevState state
	cfg       *config.Config
	client    *clickup.Client

	activeList *list.Model

	teamsList  list.Model
	spacesList list.Model
	listsList  list.Model
	tasksList  list.Model
	searchList list.Model
	fileList   list.Model

	allTeams   []clickup.Team
	allSpaces  []clickup.Space
	allFolders []clickup.Folder
	allLists   []clickup.List
	allTasks   []clickup.Task

	selectedFolder *clickup.Folder

	commentInput textarea.Model
	taskInput    textinput.Model // used for create / rename
	descInput    textarea.Model
	cmdInput     textinput.Model
	vp           viewport.Model
	renderer     *glamour.TermRenderer
	width        int
	height       int

	suggestions     []Suggestion
	filteredSuggest []Suggestion
	suggestIdx      int

	selectedTeam       string
	selectedSpace      string
	selectedList       string
	selectedTask       clickup.Task
	detailBackState    state
	taskHistory        []clickup.Task
	searchResults      []clickup.Task
	searchQuery        string
	moveCandidateLists []clickup.List
	moveTaskID         string
	teamMembers        []clickup.Member
	teamMembersTaskID  string
	parentTaskID       string // used when creating subtasks

	loading    bool
	loadingMsg string
	spinner    spinner.Model
	popupMsg   string

	selectedComments          []clickup.Comment
	commentSelectedIdx        int
	commentReturnState        state
	editingCommentID          string
	replyToCommentID          string
	replyToUser               string
	mentionSuggestions        []clickup.Member
	mentionSelectedIdx        int
	mentionQuery              string
	mentionQueryStart         int
	mentionQueryEnd           int
	pendingDeleteTaskID       string
	pendingDeleteTaskName     string
	pendingDeleteProfile      string
	pendingDeleteListID       string
	pendingDeleteListName     string
	pendingDeleteListFolderID string
	pendingDeleteSpaceID      string
	pendingDeleteSpaceName    string
	filePickerPath            string
	filePickerShowHidden      bool
	externalEditTarget        string

	// Checklist view state
	checklistViewItems     []checklistViewItem
	checklistSelectedIdx   int
	checklistEditingItem   *checklistViewItem
	checklistEditInput     textinput.Model
	checklistPendingDelete clickup.Checklist
	checklistEditOriginal  string
	checklistSelection     *checklistSelectionTarget
	currentUser            string
	currentUserID          int
	activeProfile          string
	err                    error
}

type searchQuery struct {
	Raw      string
	Text     string
	Status   string
	Assignee string
	Title    string
	ID       string
}

type checklistSelectionTarget struct {
	checklistID         string
	selectLastItem      bool
	selectLastChecklist bool
}
