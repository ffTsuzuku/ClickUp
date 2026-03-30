package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

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
	stateConfirmProfileDelete
	stateConfirmListDelete
	stateConfirmDiscardDesc
	stateFilePicker
	stateChecklist
	stateConfirmChecklistDelete
)

// Item Wrappers
type teamItem clickup.Team

func (t teamItem) Title() string       { return t.Name }
func (t teamItem) Description() string { return "Workspace (" + t.ID + ")" }
func (t teamItem) FilterValue() string { return t.Name }

type spaceItem clickup.Space

func (t spaceItem) Title() string       { return t.Name }
func (t spaceItem) Description() string { return "Space" }
func (t spaceItem) FilterValue() string { return t.Name }

type listItem clickup.List

func (t listItem) Title() string       { return t.Name }
func (t listItem) Description() string { return "List" }
func (t listItem) FilterValue() string { return t.Name }

type folderItem clickup.Folder

func (f folderItem) Title() string       { return f.Name }
func (f folderItem) Description() string { return "Folder" }
func (f folderItem) FilterValue() string { return f.Name }

type filePickerItem struct {
	Name  string
	Path  string
	IsDir bool
}

func (f filePickerItem) Title() string {
	if f.IsDir {
		return "[DIR] " + f.Name
	}
	return f.Name
}

func (f filePickerItem) Description() string {
	if f.IsDir {
		return "Folder"
	}
	return f.Path
}

func (f filePickerItem) FilterValue() string { return f.Name }

type checklistItemType int

const (
	checklistTypeHeader checklistItemType = iota
	checklistTypeItem
)

type checklistViewItem struct {
	itemType  checklistItemType
	checklist clickup.Checklist
	item      clickup.ChecklistItem
	itemIndex int
}

type checklistDeleteConfirmMsg struct {
	Checklist clickup.Checklist
}

func taskAssigneeNames(task clickup.Task) []string {
	if len(task.Assignees) == 0 {
		return nil
	}

	names := make([]string, 0, len(task.Assignees))
	seen := make(map[string]bool, len(task.Assignees))
	for _, assignee := range task.Assignees {
		if assignee.Username == "" {
			continue
		}
		key := strings.ToLower(assignee.Username)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, assignee.Username)
	}
	return names
}

func taskAssigneeDisplay(task clickup.Task, normalized bool) string {
	names := taskAssigneeNames(task)
	if len(names) == 0 {
		return "unassigned"
	}
	if normalized {
		for i := range names {
			names[i] = strings.ToLower(names[i])
		}
	}
	return strings.Join(names, ", ")
}

func taskMatchesAssignee(task clickup.Task, term string) bool {
	term = normalizeSearchValue(term)
	if term == "" {
		return true
	}

	names := taskAssigneeNames(task)
	if len(names) == 0 {
		return strings.Contains("unassigned", term) || fuzzyMatch(term, "unassigned")
	}

	for _, name := range names {
		normalizedName := normalizeSearchValue(name)
		if strings.Contains(normalizedName, term) || fuzzyMatch(term, normalizedName) {
			return true
		}
	}

	return false
}

type taskItem clickup.Task

func (t taskItem) Title() string {
	id := t.ID
	if t.CustomID != "" {
		id = t.CustomID
	}
	if t.Parent != nil {
		return fmt.Sprintf("[subtask][%s] %s", id, t.Name)
	}
	return fmt.Sprintf("[%s] %s", id, t.Name)
}
func (t taskItem) Description() string {
	assignee := taskAssigneeDisplay(clickup.Task(t), true)
	pts := "0"
	if t.Points != nil {
		pts = fmt.Sprintf("%v", *t.Points)
	}

	priority := lipgloss.NewStyle().Foreground(ColorSubtext).Render("NONE")
	if t.Priority != nil {
		pColor := t.Priority.Color
		if pColor == "" {
			pColor = "#6e7681"
		}
		priority = lipgloss.NewStyle().Foreground(lipgloss.Color(pColor)).Bold(true).Render(strings.ToUpper(t.Priority.Priority))
	}

	status := t.Status.Status
	switch strings.ToLower(status) {
	case "todo", "open":
		status = StatusTodoStyle.Render(status)
	case "in progress", "active":
		status = StatusInProgressStyle.Render(status)
	case "done", "complete", "closed":
		status = StatusDoneStyle.Render(status)
	}

	return fmt.Sprintf("Status: %s | %s | PTS: %s | PRI: %s", status, assignee, pts, priority)
}
func (t taskItem) FilterValue() string {
	assignee := taskAssigneeDisplay(clickup.Task(t), true)

	title := strings.ToLower(t.Name)
	status := strings.ToLower(t.Status.Status)

	id := t.ID
	if t.CustomID != "" {
		id = t.CustomID
	}
	idLower := strings.ToLower(id)

	return fmt.Sprintf("id:%s assignee:%s status:%s title:%s %s %s", idLower, assignee, status, title, t.Name, idLower)
}

func formatClickUpTimestamp(ts string) string {
	if ts == "" {
		return "Unknown"
	}

	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ts
	}

	return time.UnixMilli(ms).Local().Format("Jan 2, 2006 3:04 PM")
}

func (m *AppModel) stateLabel() string {
	switch m.state {
	case stateTeams:
		return "Workspaces"
	case stateSpaces:
		return "Spaces"
	case stateLists:
		return "Lists"
	case stateTasks:
		return "Tasks"
	case stateSearchResults:
		return "Search Results"
	case stateTaskDetail:
		return "Task Detail"
	case stateComment:
		return "New Comment"
	case stateCommand:
		return "Command Prompt"
	case stateHelp:
		return "Help"
	case stateCreateTask:
		return "Create Task"
	case stateMovePicker:
		return "Move Picker"
	case stateEditTitle:
		return "Edit Title"
	case stateEditDesc:
		return "Edit Description"
	case stateCreateSubtask:
		return "Create Subtask"
	case stateEditComment:
		return "Edit Comment"
	case stateConfirmProfileDelete:
		return "Confirm Profile Delete"
	case stateConfirmListDelete:
		return "Confirm List Delete"
	case stateConfirmDiscardDesc:
		return "Confirm Discard Description"
	case stateFilePicker:
		return "Attachment File Picker"
	case stateChecklist:
		return "Checklists"
	case stateConfirmChecklistDelete:
		return "Confirm Delete Checklist"
	default:
		return "Unknown"
	}
}

func (m *AppModel) recordError(context string, err error) {
	if err == nil {
		return
	}

	taskID := m.selectedTask.ID
	if m.selectedTask.CustomID != "" {
		taskID = m.selectedTask.CustomID
	}

	_ = config.AppendErrorLog(config.ErrorLogEntry{
		Time:      time.Now(),
		Context:   context,
		Message:   err.Error(),
		Profile:   m.activeProfile,
		State:     m.stateLabel(),
		Workspace: m.selectedTeamName(),
		Space:     m.selectedSpaceName(),
		List:      m.selectedListName(),
		TaskID:    taskID,
		TaskName:  m.selectedTask.Name,
	})
}

type Suggestion struct {
	Text string
	Desc string
}

func newBaseModel(cfg *config.Config) *AppModel {
	teamsList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	teamsList.Title = "Select Workspace"
	teamsList.SetShowStatusBar(false)
	teamsList.SetFilteringEnabled(false)

	spacesList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	spacesList.Title = "Select Space"
	spacesList.SetFilteringEnabled(false)

	listsList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	listsList.Title = "Select List"
	listsList.SetFilteringEnabled(false)

	tasksList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	tasksList.Title = "Tasks"
	tasksList.SetFilteringEnabled(false)

	searchList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	searchList.Title = "Search Results"
	searchList.SetFilteringEnabled(false)

	fileList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	fileList.Title = "Upload Attachment"
	fileList.SetFilteringEnabled(false)

	ci := textarea.New()
	ci.Placeholder = "Enter comment..."
	ci.SetWidth(80)
	ci.SetHeight(5)

	cmd := textinput.New()
	cmd.Placeholder = "Enter slash command (e.g. /filter) or /help..."
	cmd.Prompt = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("> ")
	cmd.CharLimit = 156
	cmd.Width = 50

	ti := textinput.New()
	ti.Placeholder = "Task name..."
	ti.CharLimit = 200

	clEdit := textinput.New()
	clEdit.Placeholder = "Item name..."
	clEdit.CharLimit = 200
	clEdit.Prompt = "  > "

	da := textarea.New()
	da.Placeholder = "Enter description..."
	da.CharLimit = 5000
	da.SetWidth(80)
	da.SetHeight(10)

	vp := viewport.New(0, 0)
	vp.Style = DetailStyle

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	m := &AppModel{
		state:                stateTeams,
		prevState:            stateTeams,
		cfg:                  cfg,
		client:               clickup.NewClient(cfg.ClickupAPIKey),
		teamsList:            teamsList,
		spacesList:           spacesList,
		listsList:            listsList,
		tasksList:            tasksList,
		searchList:           searchList,
		fileList:             fileList,
		commentInput:         ci,
		taskInput:            ti,
		descInput:            da,
		cmdInput:             cmd,
		vp:                   vp,
		renderer:             r,
		spinner:              s,
		checklistViewItems:   nil,
		checklistSelectedIdx: 0,
		checklistEditingItem: nil,
		checklistEditInput:   clEdit,
		currentUser:          "Unauthenticated",
		currentUserID:        0,
		activeProfile:        cfg.ActiveProfileName(),
	}
	m.activeList = &m.teamsList
	return m
}

func isStaleRoutingErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") ||
		strings.Contains(msg, "not authorized") ||
		strings.Contains(msg, "not found")
}

func saveClearedRoutingDefaults(cfg *config.Config) error {
	cfg.ClearRoutingDefaults()
	return config.SaveConfig(cfg)
}

func hydrateModelDefaults(m *AppModel) (string, error) {
	cfg := m.cfg
	c := m.client
	if cfg.ClickupTeamID == "" {
		return "", nil
	}

	m.selectedTeam = cfg.ClickupTeamID
	spaces, err := c.GetSpaces(cfg.ClickupTeamID)
	if err != nil {
		if isStaleRoutingErr(err) {
			if saveErr := saveClearedRoutingDefaults(cfg); saveErr != nil {
				return "", saveErr
			}
			m.selectedTeam = ""
			return "Saved routing defaults were cleared because this workspace is not accessible for the active profile.", nil
		}
		return "", err
	}
	m.allSpaces = spaces
	var sItems []list.Item
	for _, s := range spaces {
		sItems = append(sItems, spaceItem(s))
	}
	m.spacesList.SetItems(sItems)
	m.state = stateSpaces
	m.activeList = &m.spacesList

	if cfg.ClickupSpaceID == "" {
		return "", nil
	}

	m.selectedSpace = cfg.ClickupSpaceID
	hierarchy, err := c.GetSpaceLists(cfg.ClickupSpaceID)
	if err != nil {
		if isStaleRoutingErr(err) {
			cfg.ClickupSpaceID = ""
			cfg.ClickupFolderID = ""
			cfg.ClickupListID = ""
			if saveErr := config.SaveConfig(cfg); saveErr != nil {
				return "", saveErr
			}
			m.selectedSpace = ""
			return "Saved space/list defaults were cleared because they are not accessible for the active profile.", nil
		}
		return "", err
	}
	m.allFolders = hierarchy.Folders
	m.allLists = hierarchy.Lists
	m.selectedFolder = nil
	var lItems []list.Item
	for _, f := range m.allFolders {
		lItems = append(lItems, folderItem(f))
	}
	for _, l := range m.allLists {
		lItems = append(lItems, listItem(l))
	}
	m.listsList.SetItems(lItems)
	m.state = stateLists
	m.activeList = &m.listsList

	if cfg.ClickupFolderID != "" {
		foundFolder := false
		for _, f := range m.allFolders {
			if f.ID == cfg.ClickupFolderID {
				foundFolder = true
				m.selectedFolder = &f
				var items []list.Item
				for _, l := range f.Lists {
					items = append(items, listItem(l))
				}
				m.listsList.SetItems(items)
				break
			}
		}
		if !foundFolder {
			cfg.ClickupFolderID = ""
			_ = config.SaveConfig(cfg)
		}
	}

	if cfg.ClickupListID == "" {
		return "", nil
	}

	m.selectedList = cfg.ClickupListID
	tasks, err := c.GetTasksForVisibleList(cfg.ClickupTeamID, cfg.ClickupListID)
	if err != nil {
		if isStaleRoutingErr(err) {
			cfg.ClickupListID = ""
			if saveErr := config.SaveConfig(cfg); saveErr != nil {
				return "", saveErr
			}
			m.selectedList = ""
			return "Saved list default was cleared because it is not accessible for the active profile.", nil
		}
		return "", err
	}
	m.allTasks = tasks
	m.applyTaskFilter("")
	m.state = stateTasks
	m.activeList = &m.tasksList
	return "", nil
}

func bootstrapModel(cfg *config.Config) *AppModel {
	m := newBaseModel(cfg)
	hasAuth := cfg.ClickupAPIKey != "" && cfg.ClickupAPIKey != "NO_TOKEN"
	if !hasAuth {
		return m
	}

	u, err := m.client.GetUser()
	if err == nil {
		m.currentUser = u.Username
		m.currentUserID = u.ID
	}

	teams, err := m.client.GetTeams()
	if err == nil {
		m.allTeams = teams
		var items []list.Item
		for _, t := range teams {
			items = append(items, teamItem(t))
		}
		m.teamsList.SetItems(items)
	}

	_, _ = hydrateModelDefaults(m)
	return m
}

func reloadProfileCmd(cfg *config.Config, width, height int, popup string) tea.Cmd {
	return func() tea.Msg { return profileReloadStartMsg{Cfg: cfg, Width: width, Height: height, Popup: popup} }
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
	parentTaskID       string // used when creating subtasks

	loading    bool
	loadingMsg string
	spinner    spinner.Model
	popupMsg   string

	selectedComments          []clickup.Comment
	editingCommentID          string
	replyToCommentID          string
	replyToUser               string
	pendingDeleteProfile      string
	pendingDeleteListID       string
	pendingDeleteListName     string
	pendingDeleteListFolderID string
	filePickerPath            string
	filePickerShowHidden      bool
	externalEditTarget        string

	// Checklist view state
	checklistViewItems     []checklistViewItem
	checklistSelectedIdx   int
	checklistEditingItem   *checklistViewItem
	checklistEditInput     textinput.Model
	checklistPendingDelete string
	checklistEditOriginal  string

	currentUser   string
	currentUserID int
	activeProfile string
	err           error
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
type editorFinishedMsg struct {
	content string
	err     error
}
type searchResultsMsg struct {
	Query string
	Tasks []clickup.Task
}
type checklistItemUpdatedMsg struct{}
type checklistCreatedMsg struct{}
type checklistDeletedMsg struct{}

type searchQuery struct {
	Raw      string
	Text     string
	Status   string
	Assignee string
	Title    string
	ID       string
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

func popupWithErr(success, phase string, err error) string {
	if err == nil {
		return success
	}
	return fmt.Sprintf("Error %s: %v", phase, err)
}

func popupWithWarning(base, warning string) string {
	if warning == "" {
		return base
	}
	if base == "" {
		return warning
	}
	return base + " " + warning
}

func normalizeSearchValue(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return s
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

	// Sort top-level oldest to newest (API returns newest first)
	for i, j := 0, len(topLevel)-1; i < j; i, j = i+1, j-1 {
		topLevel[i], topLevel[j] = topLevel[j], topLevel[i]
	}

	for i := range topLevel {
		if topLevel[i].ReplyCount > 0 {
			replies, _ := c.GetCommentReplies(topLevel[i].ID)
			// Sort replies oldest to newest as well
			for i, j := 0, len(replies)-1; i < j; i, j = i+1, j-1 {
				replies[i], replies[j] = replies[j], replies[i]
			}
			topLevel[i].Replies = replies
		}
	}
	return flattenComments(topLevel), nil
}

func createTaskCmd(c *clickup.Client, listID, name string, userID int) tea.Cmd {
	return func() tea.Msg {
		var assignees []int
		if userID != 0 {
			assignees = []int{userID}
		}
		task, err := c.CreateTask(listID, name, assignees)
		if err != nil {
			return errMsg(err)
		}
		return taskCreatedMsg(*task)
	}
}

func createSubtaskCmd(c *clickup.Client, listID, parentID, name string, userID int) tea.Cmd {
	return func() tea.Msg {
		var assignees []int
		if userID != 0 {
			assignees = []int{userID}
		}
		task, err := c.CreateSubtask(listID, parentID, name, assignees)
		if err != nil {
			return errMsg(err)
		}
		return taskCreatedMsg(*task)
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

func addCommentCmd(c *clickup.Client, taskID, comment, parentID string) tea.Cmd {
	return func() tea.Msg {
		if err := c.AddComment(taskID, comment, parentID); err != nil {
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

func editCommentCmd(c *clickup.Client, commentID, text string) tea.Cmd {
	return func() tea.Msg {
		if err := c.UpdateComment(commentID, text); err != nil {
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
		// Refresh task detail to show updated priority
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
		return commentAddedMsg{}
	}
}

func openAttachmentURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", url).Run()
		return nil
	}
}

func attachmentOpenURL(a clickup.Attachment) string {
	if a.URLWithQuery != "" {
		return a.URLWithQuery
	}
	return a.URL
}

func expandUserPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
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

func fetchTeamMembersCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		members, err := c.GetTeamMembers(teamID)
		if err != nil {
			return errMsg(err)
		}
		return teamMembersMsg(members)
	}
}

func parseSearchQuery(raw string) searchQuery {
	raw = strings.TrimSpace(raw)
	parts := strings.Fields(raw)
	q := searchQuery{Raw: raw}
	var textParts []string
	for _, part := range parts {
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "status:"):
			q.Status = normalizeSearchValue(strings.TrimPrefix(lower, "status:"))
		case strings.HasPrefix(lower, "assignee:"):
			q.Assignee = normalizeSearchValue(strings.TrimPrefix(lower, "assignee:"))
		case strings.HasPrefix(lower, "title:"):
			q.Title = normalizeSearchValue(strings.TrimPrefix(lower, "title:"))
		case strings.HasPrefix(lower, "id:"):
			q.ID = normalizeSearchValue(strings.TrimPrefix(lower, "id:"))
		default:
			textParts = append(textParts, part)
		}
	}
	q.Text = normalizeSearchValue(strings.Join(textParts, " "))
	return q
}

func matchesSearchFilters(q searchQuery, task clickup.Task) bool {
	status := normalizeSearchValue(task.Status.Status)
	title := normalizeSearchValue(task.Name)
	id := normalizeSearchValue(task.ID)
	if task.CustomID != "" {
		id = normalizeSearchValue(task.CustomID)
	}

	if q.Status != "" && !strings.Contains(status, q.Status) && !fuzzyMatch(q.Status, status) {
		return false
	}
	if q.Assignee != "" && !taskMatchesAssignee(task, q.Assignee) {
		return false
	}
	if q.Title != "" && !strings.Contains(title, q.Title) && !fuzzyMatch(q.Title, title) {
		return false
	}
	if q.ID != "" && !strings.Contains(id, q.ID) && !fuzzyMatch(q.ID, id) {
		return false
	}
	return true
}

func scoreSearchMatch(q searchQuery, task clickup.Task) int {
	query := q.Text
	if query == "" {
		if q.ID != "" || q.Title != "" || q.Status != "" || q.Assignee != "" {
			return 1
		}
		return 0
	}

	id := normalizeSearchValue(task.ID)
	if task.CustomID != "" {
		id = normalizeSearchValue(task.CustomID)
	}
	title := normalizeSearchValue(task.Name)
	desc := normalizeSearchValue(task.Desc)

	switch {
	case id == query:
		return 1000
	case strings.HasPrefix(id, query):
		return 900
	case strings.HasPrefix(title, query):
		return 800
	case strings.Contains(title, query):
		return 700
	case fuzzyMatch(query, title):
		return 600
	case strings.Contains(id, query):
		return 500
	case strings.Contains(desc, query):
		return 300
	case fuzzyMatch(query, id):
		return 200
	default:
		return 0
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

	// Write current content to a temp file
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

func InitialModel() *AppModel {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		cfg = &config.Config{ClickupAPIKey: "NO_TOKEN"}
	}
	m := bootstrapModel(cfg)
	m.activeProfile = cfg.ActiveProfileName()
	return m
}

func (m *AppModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *AppModel) updateLayout() {
	h, v := BaseStyle.GetFrameSize()

	headerH := lipgloss.Height(m.renderHeader())
	breadcrumbH := 0
	if breadcrumb := m.breadcrumb(); breadcrumb != "" {
		breadcrumbH = lipgloss.Height(breadcrumb) + 2
	}

	bottomBarH := 3
	auxH := 0
	switch m.state {
	case stateCreateTask, stateCreateSubtask, stateEditTitle:
		auxH = 3
	case stateComment, stateEditComment:
		auxH = 4
	case stateTaskDetail:
		auxH = 1
	}

	contentH := m.height - v - headerH - breadcrumbH - bottomBarH - auxH

	if m.state == stateCommand {
		menuH := len(m.filteredSuggest)
		if menuH > 0 {
			contentH -= (menuH + 1) // +1 for the newline spacing
		}
	}

	if contentH < 5 {
		contentH = 5
	}

	m.teamsList.SetSize(m.width-h, contentH)
	m.spacesList.SetSize(m.width-h, contentH)
	m.listsList.SetSize(m.width-h, contentH)
	m.tasksList.SetSize(m.width-h, contentH)
	m.searchList.SetSize(m.width-h, contentH)
	m.fileList.SetSize(m.width-h, contentH)
	m.vp.Width = m.width - h
	m.vp.Height = contentH
	if m.state == stateEditDesc {
		m.refreshEditDescLayout()
	} else {
		m.descInput.SetWidth(m.width - h)
		m.descInput.SetHeight(contentH)
	}
}

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
		m.loading = false
		m.selectedTask = *msg.Task
		m.detailBackState = msg.BackState

		m.selectedComments = msg.Comments

		if m.state != stateTaskDetail {
			m.taskHistory = nil
		}
		m.state = stateTaskDetail
		m.prevState = msg.BackState
		m.updateViewportContent()
		return m, nil
	case attachmentUploadedMsg:
		m.loading = false
		m.selectedTask = *msg.Task
		m.selectedComments = msg.Comments
		m.state = stateTaskDetail
		m.prevState = msg.BackState
		m.popupMsg = msg.Popup
		m.updateViewportContent()
		return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
	case taskCreatedMsg:
		m.loading = false
		newTask := clickup.Task(msg)
		m.allTasks = append([]clickup.Task{newTask}, m.allTasks...)
		m.applyTaskFilter("")
		// Navigate directly into the new ticket (unless it's a subtask — stay in parent)
		if newTask.Parent == nil {
			m.selectedTask = newTask
			m.taskHistory = nil
			m.state = stateTaskDetail
			m.updateViewportContent()
		} else {
			// Refresh the parent task detail view to show new subtask
			m.state = stateTaskDetail
			m.updateViewportContent()
		}
		return m, nil
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
				if err := m.client.UpdateDescription(m.selectedTask.ID, content); err == nil {
					m.selectedTask.Desc = content
					m.selectedTask.MarkdownDescription = content
					for i, t := range m.allTasks {
						if t.ID == m.selectedTask.ID {
							m.allTasks[i].Desc = content
							m.allTasks[i].MarkdownDescription = content
							break
						}
					}
				}
			} else if m.externalEditTarget == "comment" || m.state == stateEditComment {
				m.loading = true
				m.loadingMsg = "Updating comment..."
				cmds = append(cmds, tea.Batch(m.spinner.Tick, editCommentCmd(m.client, m.editingCommentID, content)))
			} else if m.externalEditTarget == "new_comment" || m.state == stateComment {
				m.loading = true
				m.loadingMsg = "Adding comment..."
				cmds = append(cmds, tea.Batch(m.spinner.Tick, addCommentCmd(m.client, m.selectedTask.ID, content, m.replyToCommentID)))
			}
		}
		m.externalEditTarget = ""
		m.state = stateTaskDetail
		m.updateViewportContent()
		return m, tea.Batch(cmds...)
	case commentAddedMsg:
		m.replyToCommentID = ""
		m.replyToUser = ""
		m.popupMsg = "Comment added!"
		return m, tea.Batch(
			fetchCommentsCmd(m.client, m.selectedTask.ID),
			tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} }),
		)
	case commentsMsg:
		m.loading = false
		m.selectedComments = msg
		m.updateViewportContent()
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
	case stateConfirmProfileDelete:
		return m.updateConfirmProfileDelete(msg)
	case stateConfirmListDelete:
		return m.updateConfirmListDelete(msg)
	case stateConfirmDiscardDesc:
		return m.updateConfirmDiscardDesc(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) getSubtasks(parentID string) []clickup.Task {
	var res []clickup.Task
	for _, t := range m.allTasks {
		if t.Parent != nil && *t.Parent == parentID {
			res = append(res, t)
		}
	}
	return res
}

func (m *AppModel) editableDescription() string {
	if m.selectedTask.MarkdownDescription != "" {
		return m.selectedTask.MarkdownDescription
	}
	return m.selectedTask.Desc
}

func (m *AppModel) hasUnsavedDescriptionChanges() bool {
	current := strings.TrimRight(m.descInput.Value(), "\n")
	original := strings.TrimRight(m.editableDescription(), "\n")
	return current != original
}

func (m *AppModel) refreshEditDescLayout() {
	h, _ := BaseStyle.GetFrameSize()
	contentW := m.width - h
	if contentW < 40 {
		contentW = 40
	}
	paneGap := 1
	editorPaneW := (contentW - paneGap) / 2
	if editorPaneW < 20 {
		editorPaneW = 20
	}
	editorChrome := 6
	editorW := editorPaneW - editorChrome
	if editorW < 10 {
		editorW = 10
	}
	m.descInput.SetWidth(editorW)
	m.descInput.SetHeight(m.vp.Height - 4)
	if m.vp.Height < 8 {
		m.descInput.SetHeight(5)
	}
}

func (m *AppModel) renderEditDesc() string {
	paneGap := 1
	contentW := m.vp.Width
	if contentW < 40 {
		contentW = 40
	}
	editorPaneW := (contentW - paneGap) / 2
	previewPaneW := contentW - editorPaneW - paneGap
	if previewPaneW < 20 {
		previewPaneW = 20
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Height(m.vp.Height)

	editorPanel := panelStyle.Width(editorPaneW)
	previewPanel := panelStyle.Width(previewPaneW)

	previewWidth := previewPaneW - 4
	if previewWidth < 10 {
		previewWidth = 10
	}

	previewContent := strings.TrimSpace(m.descInput.Value())
	if previewContent == "" {
		previewContent = "_Nothing to preview yet._"
	}

	renderedPreview := previewContent
	if m.renderer != nil {
		if out, err := m.renderer.Render(previewContent); err == nil {
			renderedPreview = out
		}
	}

	left := editorPanel.Render(SectionHeaderStyle.Render("MARKDOWN") + "\n\n" + m.descInput.View())
	right := previewPanel.Render(SectionHeaderStyle.Render("PREVIEW") + "\n\n" + renderedPreview)

	header := TitleStyle.Render(fmt.Sprintf("Editing: %s", m.selectedTask.Name))
	hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("Ctrl+S to save | Esc to cancel")

	return header + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", paneGap), right) + "\n" + hint
}

func (m *AppModel) filterSuggestions() {
	v := strings.ToLower(m.cmdInput.Value())
	words := strings.Split(v, " ")

	m.filteredSuggest = nil
	for _, s := range m.suggestions {
		text := strings.ToLower(s.Text)
		match := true
		for _, w := range words {
			if w != "" && !strings.Contains(text, w) {
				match = false
				break
			}
		}
		if match {
			m.filteredSuggest = append(m.filteredSuggest, s)
			if len(m.filteredSuggest) >= 10 { // Limit to top 10 matches
				break
			}
		}
	}
	m.suggestIdx = 0
	m.updateLayout()
}

func (m *AppModel) updateCommandSuggestions() {
	var sugs []Suggestion

	sugs = append(sugs, Suggestion{"/clear", "Clear active list filters"})
	sugs = append(sugs, Suggestion{"/help", "Show help documentation"})
	sugs = append(sugs, Suggestion{"/ticket ", "Open a ticket directly by ID"})
	sugs = append(sugs, Suggestion{"/search ", "Search tickets across the workspace"})
	sugs = append(sugs, Suggestion{"/space create ", "Create a new Space in the current Workspace"})
	sugs = append(sugs, Suggestion{"/space rename ", "Rename the highlighted Space"})
	sugs = append(sugs, Suggestion{"/list create ", "Create a new List in the current Folder or Space"})
	sugs = append(sugs, Suggestion{"/list rename ", "Rename the highlighted List"})
	sugs = append(sugs, Suggestion{"/list delete", "Delete the highlighted List"})
	sugs = append(sugs, Suggestion{"/profile list", "List available ClickUp profiles"})
	sugs = append(sugs, Suggestion{"/profile create ", "Create a new empty profile and switch to it"})
	sugs = append(sugs, Suggestion{"/profile switch ", "Switch to another profile"})
	sugs = append(sugs, Suggestion{"/profile delete ", "Delete a profile with typed confirmation"})
	sugs = append(sugs, Suggestion{"/profile save ", "Save current settings as a named profile"})
	sugs = append(sugs, Suggestion{"/profile token ", "Set the API key for the active profile"})
	for _, profileName := range m.cfg.ProfileNames() {
		sugs = append(sugs, Suggestion{"/profile switch " + profileName, "Switch to profile " + profileName})
		sugs = append(sugs, Suggestion{"/profile delete " + profileName, "Delete profile " + profileName})
	}
	sugs = append(sugs, Suggestion{"/search status:in progress", "Search tickets filtered by status"})
	sugs = append(sugs, Suggestion{"/search assignee:deep api", "Search tickets filtered by assignee plus text"})

	if m.prevState == stateTeams || m.prevState == stateSpaces || m.prevState == stateLists {
		sugs = append(sugs, Suggestion{"/default set", "Set the currently highlighted item as your default routing"})
	}
	sugs = append(sugs, Suggestion{"/default user ", "Set a default assignee filter (e.g. /default user deep)"})
	sugs = append(sugs, Suggestion{"/default user clear", "Clear default assignee filter"})
	sugs = append(sugs, Suggestion{"/default clear", "Clear all default automatic routing"})

	if m.prevState == stateTasks && len(m.allTasks) > 0 {
		sugs = append(sugs, Suggestion{"/filter assignee", "Filter by a specific assignee"})
		sugs = append(sugs, Suggestion{"/filter status", "Filter by task status"})
		sugs = append(sugs, Suggestion{"/filter title", "Filter by task title text"})
		sugs = append(sugs, Suggestion{"/filter id", "Filter by task ID"})

		assignees := make(map[string]bool)
		statuses := make(map[string]bool)
		for _, t := range m.allTasks {
			statuses[strings.ToLower(t.Status.Status)] = true
			for _, assignee := range taskAssigneeNames(t) {
				assignees[strings.ToLower(assignee)] = true
			}
		}
		for a := range assignees {
			sugs = append(sugs, Suggestion{"/filter assignee " + a, "Show tasks assigned to " + a})
		}
		for s := range statuses {
			sugs = append(sugs, Suggestion{"/filter status " + s, "Show tasks in " + s + " status"})
		}
	} else if m.prevState == stateTaskDetail {
		sugs = append(sugs, Suggestion{"/status ", "Change ticket status"})
		sugs = append(sugs, Suggestion{"/points ", "Set Story Points (e.g. /points 3)"})
		sugs = append(sugs, Suggestion{"/share", "Copy ticket URL to clipboard"})
		sugs = append(sugs, Suggestion{"/delete", "Delete this ticket permanently"})
		sugs = append(sugs, Suggestion{"/move", "Move this ticket to another list"})
		sugs = append(sugs, Suggestion{"/assign ", "Change assignee (e.g. /assign deep)"})
		sugs = append(sugs, Suggestion{"/title", "Edit the ticket title"})
		sugs = append(sugs, Suggestion{"/desc", "Edit the ticket description (inline)"})
		sugs = append(sugs, Suggestion{"/copydesc", "Copy the ticket description to your clipboard"})
		sugs = append(sugs, Suggestion{"/editext", "Edit description in $EDITOR (vim etc)"})
		sugs = append(sugs, Suggestion{"/subtask", "Add a subtask to this ticket"})
		sugs = append(sugs, Suggestion{"/checklist add ", "Create a checklist on this ticket"})
		sugs = append(sugs, Suggestion{"/checklist rename ", "Rename a checklist by number"})
		sugs = append(sugs, Suggestion{"/checklist delete ", "Delete a checklist by number"})
		sugs = append(sugs, Suggestion{"/checklist item add ", "Add an item to a checklist"})
		sugs = append(sugs, Suggestion{"/checklist item rename ", "Rename a checklist item"})
		sugs = append(sugs, Suggestion{"/checklist item toggle ", "Toggle a checklist item"})
		sugs = append(sugs, Suggestion{"/checklist item delete ", "Delete a checklist item"})
		sugs = append(sugs, Suggestion{"/attach open ", "Open an attachment preview in your browser by number (e.g. /attach open 1)"})
		sugs = append(sugs, Suggestion{"/attach download ", "Download an attachment by number (e.g. /attach download 1)"})
		sugs = append(sugs, Suggestion{"/attach share ", "Copy an attachment URL to your clipboard by number (e.g. /attach share 1)"})
		sugs = append(sugs, Suggestion{"/attach upload", "Open a file browser to upload an attachment"})
		sugs = append(sugs, Suggestion{"/comment edit ", "Edit a comment by its number (e.g. /comment edit 1)"})
		sugs = append(sugs, Suggestion{"/priority ", "Set task priority (urgent, high, normal, low, none)"})
		sugs = append(sugs, Suggestion{"/priority urgent", "Set priority to Urgent"})
		sugs = append(sugs, Suggestion{"/priority high", "Set priority to High"})
		sugs = append(sugs, Suggestion{"/priority normal", "Set priority to Normal"})
		sugs = append(sugs, Suggestion{"/priority low", "Set priority to Low"})
		sugs = append(sugs, Suggestion{"/priority none", "Clear task priority"})
		sugs = append(sugs, Suggestion{"/comment delete ", "Delete a comment by its number (e.g. /comment delete 1)"})
		sugs = append(sugs, Suggestion{"/comment reply ", "Reply to a comment by its number (e.g. /comment reply 1)"})

		// Suggest all known workspace members
		for _, member := range m.teamMembers {
			sugs = append(sugs, Suggestion{"/assign " + strings.ToLower(member.User.Username), "Assign to " + member.User.Username})
		}
		if len(m.teamMembers) == 0 {
			// Fall back to cached task assignees until member list is loaded
			seenUsers := make(map[string]bool)
			for _, t := range m.allTasks {
				for _, a := range t.Assignees {
					if !seenUsers[a.Username] {
						seenUsers[a.Username] = true
						sugs = append(sugs, Suggestion{"/assign " + strings.ToLower(a.Username), "Assign to " + a.Username})
					}
				}
			}
		}

		for _, s := range m.availableStatuses() {
			sugs = append(sugs, Suggestion{"/status " + s, "Set status to " + s})
		}
	} else if m.prevState == stateTeams {
		sugs = append(sugs, Suggestion{"/filter", "Filter workspaces by name"})
		for _, t := range m.allTeams {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find workspace " + t.Name})
		}
	} else if m.prevState == stateSpaces {
		sugs = append(sugs, Suggestion{"/filter", "Filter spaces by name"})
		sugs = append(sugs, Suggestion{"/space rename ", "Rename the highlighted Space"})
		for _, t := range m.allSpaces {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find space " + t.Name})
		}
	} else if m.prevState == stateLists {
		sugs = append(sugs, Suggestion{"/filter", "Filter lists by name"})
		sugs = append(sugs, Suggestion{"/list rename ", "Rename the highlighted List"})
		sugs = append(sugs, Suggestion{"/list delete", "Delete the highlighted List"})
		for _, t := range m.allLists {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find list " + t.Name})
		}
	}

	m.suggestions = sugs
	m.filterSuggestions()
}

func fuzzyMatch(term, target string) bool {
	term = strings.ToLower(term)
	target = strings.ToLower(target)

	tIdx := 0
	for i := 0; i < len(term); i++ {
		found := false
		for tIdx < len(target) {
			if term[i] == target[tIdx] {
				found = true
				tIdx++
				break
			}
			tIdx++
		}
		if !found {
			return false
		}
	}
	return true
}

func (m *AppModel) applyHierarchyFilter(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	switch m.prevState {
	case stateTeams:
		var items []list.Item
		for _, t := range m.allTeams {
			if query == "" || fuzzyMatch(query, t.Name) {
				items = append(items, teamItem(t))
			}
		}
		m.teamsList.SetItems(items)
	case stateSpaces:
		var items []list.Item
		for _, s := range m.allSpaces {
			if query == "" || fuzzyMatch(query, s.Name) {
				items = append(items, spaceItem(s))
			}
		}
		m.spacesList.SetItems(items)
	case stateLists:
		var items []list.Item
		for _, l := range m.allLists {
			if query == "" || fuzzyMatch(query, l.Name) {
				items = append(items, listItem(l))
			}
		}
		m.listsList.SetItems(items)
	case stateTasks:
		m.applyTaskFilter(query)
	}
}

func (m *AppModel) applyTaskFilter(query string) {
	var items []list.Item
	query = strings.ToLower(strings.TrimSpace(query))

	totalPoints := 0.0
	defaultUser := strings.ToLower(m.cfg.ClickupUserName)

	for _, t := range m.allTasks {
		// If a default user is set, heavily prioritize it unless an explicit assignee override is typed
		if defaultUser != "" && !strings.HasPrefix(query, "assignee ") && defaultUser != "clear" {
			if !taskMatchesAssignee(t, defaultUser) {
				continue
			}
		}

		if query == "" {
			items = append(items, taskItem(t))
			if t.Points != nil {
				totalPoints += *t.Points
			}
			continue
		}

		status := strings.ToLower(t.Status.Status)
		title := strings.ToLower(t.Name)

		id := t.ID
		if t.CustomID != "" {
			id = t.CustomID
		}
		idLower := strings.ToLower(id)

		if strings.HasPrefix(query, "assignee ") {
			term := strings.TrimPrefix(query, "assignee ")
			if taskMatchesAssignee(t, term) {
				items = append(items, taskItem(t))
			}
		} else if strings.HasPrefix(query, "status ") {
			term := strings.TrimPrefix(query, "status ")
			if fuzzyMatch(term, status) {
				items = append(items, taskItem(t))
			}
		} else if strings.HasPrefix(query, "title ") {
			term := strings.TrimPrefix(query, "title ")
			if fuzzyMatch(term, title) {
				items = append(items, taskItem(t))
			}
		} else if strings.HasPrefix(query, "id ") {
			term := strings.TrimPrefix(query, "id ")
			if fuzzyMatch(term, idLower) {
				items = append(items, taskItem(t))
			}
		} else {
			if fuzzyMatch(query, title) || taskMatchesAssignee(t, query) || fuzzyMatch(query, status) || fuzzyMatch(query, idLower) {
				items = append(items, taskItem(t))
				if t.Points != nil {
					totalPoints += *t.Points
				}
			}
		}
	}

	m.tasksList.Title = fmt.Sprintf("Tasks (Total Points: %v)", totalPoints)
	m.tasksList.SetItems(items)
}

func (m *AppModel) availableStatuses() []string {
	seen := make(map[string]bool)
	var statuses []string

	add := func(status string) {
		status = strings.TrimSpace(status)
		if status == "" {
			return
		}
		key := strings.ToLower(status)
		if seen[key] {
			return
		}
		seen[key] = true
		statuses = append(statuses, status)
	}

	for _, s := range m.allSpaces {
		if s.ID == m.selectedSpace {
			for _, st := range s.Statuses {
				add(st.Status)
			}
			break
		}
	}

	for _, t := range m.allTasks {
		add(t.Status.Status)
	}

	add(m.selectedTask.Status.Status)

	sort.Slice(statuses, func(i, j int) bool {
		return strings.ToLower(statuses[i]) < strings.ToLower(statuses[j])
	})

	return statuses
}

func (m *AppModel) openFilePicker(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = home
	}

	path = expandUserPath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	var dirs []filePickerItem
	var files []filePickerItem

	parent := filepath.Dir(absPath)
	if parent != absPath {
		dirs = append(dirs, filePickerItem{Name: "..", Path: parent, IsDir: true})
	}

	for _, entry := range entries {
		if !m.filePickerShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		item := filePickerItem{
			Name:  entry.Name(),
			Path:  filepath.Join(absPath, entry.Name()),
			IsDir: entry.IsDir(),
		}
		if item.IsDir {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name) })

	items := make([]list.Item, 0, len(dirs)+len(files))
	for _, item := range dirs {
		items = append(items, item)
	}
	for _, item := range files {
		items = append(items, item)
	}

	m.filePickerPath = absPath
	m.fileList.Title = "Upload Attachment: " + absPath
	m.fileList.SetItems(items)
	m.fileList.Select(0)
	m.state = stateFilePicker
	m.activeList = &m.fileList
	return nil
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
			m.state = stateComment
			m.commentInput.Focus()
			return m, textinput.Blink
		case "T":
			m.state = stateEditTitle
			m.taskInput.SetValue(m.selectedTask.Name)
			m.taskInput.Focus()
			m.taskInput.SetCursor(len(m.taskInput.Value()))
			return m, textinput.Blink
		case "e":
			m.state = stateEditDesc
			m.descInput.SetValue(m.editableDescription())
			m.refreshEditDescLayout()
			m.descInput.Focus()
			return m, textarea.Blink
		case "E":
			// Open in external editor
			m.externalEditTarget = "description"
			return m, openExternalEditorCmd(m.editableDescription())
		case "D":
			return m, m.copyTaskDescription()
		case "t":
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
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateComment(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			v := m.commentInput.Value()
			if v != "" {
				m.commentInput.SetValue("")
				m.commentInput.Blur()
				m.loading = true
				m.loadingMsg = "Adding comment..."
				return m, tea.Batch(m.spinner.Tick, addCommentCmd(m.client, m.selectedTask.ID, v, m.replyToCommentID))
			}
			return m, nil
		case "ctrl+e":
			m.externalEditTarget = "new_comment"
			return m, openExternalEditorCmd(m.commentInput.Value())
		case "esc":
			m.commentInput.SetValue("")
			m.commentInput.Blur()
			m.state = stateTaskDetail
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	return m, cmd
}

func (m *AppModel) updateEditComment(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			v := m.commentInput.Value()
			if v != "" {
				m.commentInput.SetValue("")
				m.commentInput.Blur()
				m.state = stateTaskDetail
				m.popupMsg = "Updating comment..."
				return m, tea.Batch(m.spinner.Tick, editCommentCmd(m.client, m.editingCommentID, v))
			}
			return m, nil
		case "ctrl+e":
			m.externalEditTarget = "comment"
			return m, openExternalEditorCmd(m.commentInput.Value())
		case "esc":
			m.commentInput.SetValue("")
			m.commentInput.Blur()
			m.state = stateTaskDetail
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
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
				return m, tea.Batch(m.spinner.Tick, createTaskCmd(m.client, m.selectedList, name, m.currentUserID))
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
				return m, tea.Batch(m.spinner.Tick, createSubtaskCmd(m.client, m.selectedList, m.parentTaskID, name, m.currentUserID))
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
			if err := m.client.UpdateDescription(m.selectedTask.ID, desc); err == nil {
				m.selectedTask.Desc = desc
				m.selectedTask.MarkdownDescription = desc
				for i, t := range m.allTasks {
					if t.ID == m.selectedTask.ID {
						m.allTasks[i].Desc = desc
						m.allTasks[i].MarkdownDescription = desc
						break
					}
				}
			}
			m.updateViewportContent()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.descInput, cmd = m.descInput.Update(msg)
	return m, cmd
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
				// Autocomplete value
				m.cmdInput.SetValue(m.filteredSuggest[m.suggestIdx].Text)
				m.cmdInput.SetCursor(len(m.cmdInput.Value()))
				m.filterSuggestions()
			}
			return m, nil
		case tea.KeyEnter:
			val := m.cmdInput.Value()
			// Use selected suggestion text if not empty
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
			} else if strings.HasPrefix(val, "/profile list") {
				names := m.cfg.ProfileNames()
				for i, name := range names {
					if name == m.cfg.ActiveProfileName() {
						names[i] = "*" + name
					}
				}
				m.popupMsg = "Profiles: " + strings.Join(names, ", ")
				return m, tea.Tick(time.Second*3, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
			} else if strings.HasPrefix(val, "/profile create ") {
				name := strings.TrimSpace(strings.TrimPrefix(val, "/profile create "))
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
			} else if strings.HasPrefix(val, "/desc") {
				if m.prevState == stateTaskDetail {
					m.state = stateEditDesc
					m.descInput.SetValue(m.editableDescription())
					m.refreshEditDescLayout()
					m.descInput.Focus()
					return m, textarea.Blink
				}
			} else if strings.HasPrefix(val, "/title") {
				if m.prevState == stateTaskDetail {
					m.state = stateEditTitle
					m.taskInput.SetValue(m.selectedTask.Name)
					m.taskInput.Focus()
					m.taskInput.SetCursor(len(m.taskInput.Value()))
					return m, textinput.Blink
				}
			} else if strings.HasPrefix(val, "/copydesc") {
				if m.prevState == stateTaskDetail {
					return m, m.copyTaskDescription()
				}
			} else if strings.HasPrefix(val, "/editext") {
				if m.prevState == stateTaskDetail {
					m.externalEditTarget = "description"
					return m, openExternalEditorCmd(m.editableDescription())
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
										if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, name, item.Resolved); err != nil {
											return errMsg(err)
										}
										return refreshTaskDetailCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState)()
									})
								case "toggle":
									m.loading = true
									m.loadingMsg = "Updating checklist item..."
									return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
										if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, item.Name, !item.Resolved); err != nil {
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
								m.loading = true
								m.loadingMsg = "Deleting comment..."
								m.state = stateTaskDetail
								return m, tea.Batch(m.spinner.Tick, deleteCommentCmd(m.client, comment.ID))
							} else if action == "edit" {
								m.state = stateEditComment
								m.editingCommentID = comment.ID
								m.commentInput.SetValue(comment.CommentText)
								m.commentInput.Focus()
								return m, textinput.Blink
							} else if action == "reply" {
								if comment.Parent != nil && *comment.Parent != "" {
									m.popupMsg = "Error: Cannot reply to a reply"
									return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
								}
								m.replyToCommentID = comment.ID
								m.replyToUser = comment.User.Username
								m.state = stateComment
								m.commentInput.Focus()
								return m, nil
							}
						}
					}
				}
			} else if strings.HasPrefix(val, "/assign ") {
				if m.prevState == stateTaskDetail {
					// Kick off background member fetch if not already done
					if len(m.teamMembers) == 0 && m.selectedTeam != "" {
						go func() {}() // no-op, fetch happens below
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
						// Fallback: search task cache
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
						m.cfg.ClickupFolderID = "" // Clear folder if list is set
						m.cfg.ClickupSpaceID = m.selectedSpace
						m.cfg.ClickupTeamID = m.selectedTeam
						config.SaveConfig(m.cfg)
						m.popupMsg = "Default set successfully"
					} else if i, ok := m.activeList.SelectedItem().(folderItem); ok {
						m.cfg.ClickupFolderID = i.ID
						m.cfg.ClickupListID = "" // Clear list if folder is set
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

func (m *AppModel) updateViewportContent() {
	var b strings.Builder

	id := m.selectedTask.ID
	if m.selectedTask.CustomID != "" {
		id = m.selectedTask.CustomID
	}

	// Title & Status
	b.WriteString(TitleStyle.Render(fmt.Sprintf("[%s] %s", id, m.selectedTask.Name)) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("Status: " + m.selectedTask.Status.Status))

	if m.selectedTask.Parent != nil {
		parentID := *m.selectedTask.Parent
		for _, t := range m.allTasks {
			if t.ID == parentID && t.CustomID != "" {
				parentID = t.CustomID
				break
			}
		}
		b.WriteString(LabelStyle.Render(" | Parent: ") + ColorSecondaryStyle.Render(parentID))
	}
	b.WriteString("\n\n")

	divider := lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", m.width-10))
	b.WriteString(divider + "\n\n")

	// Metadata
	assignee := taskAssigneeDisplay(m.selectedTask, false)
	if assignee == "unassigned" {
		assignee = "Unassigned"
	}

	priority := "None"
	if m.selectedTask.Priority != nil {
		p := m.selectedTask.Priority
		pColor := p.Color
		if pColor == "" {
			pColor = "#6e7681"
		}
		priority = lipgloss.NewStyle().Foreground(lipgloss.Color(pColor)).Bold(true).Render(strings.ToUpper(p.Priority))
	}

	pts := "0"
	if m.selectedTask.Points != nil {
		pts = fmt.Sprintf("%v", *m.selectedTask.Points)
	}

	creator := "Unknown"
	if m.selectedTask.Creator.Username != "" {
		creator = m.selectedTask.Creator.Username
	}

	b.WriteString(LabelStyle.Width(15).Render("Assignees:") + assignee + "\n")
	b.WriteString(LabelStyle.Width(15).Render("Created:") + formatClickUpTimestamp(m.selectedTask.DateCreated) + "\n")
	b.WriteString(LabelStyle.Width(15).Render("Created By:") + creator + "\n")
	b.WriteString(LabelStyle.Width(15).Render("Priority:") + priority + "\n")
	b.WriteString(LabelStyle.Width(15).Render("Story Points:") + pts + "\n\n")

	b.WriteString(divider + "\n\n")

	// Description
	b.WriteString(SectionHeaderStyle.Render("DESCRIPTION") + "\n")
	desc := m.selectedTask.MarkdownDescription
	if desc == "" {
		desc = m.selectedTask.Desc
	}

	if desc == "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("No description provided.") + "\n\n")
	} else {
		out, err := m.renderer.Render(desc)
		if err != nil {
			b.WriteString(desc + "\n\n")
		} else {
			b.WriteString(out + "\n")
		}
	}

	b.WriteString(divider + "\n\n")

	// Subtasks
	b.WriteString(SectionHeaderStyle.Render("SUBTASKS") + "\n")
	subtasks := m.getSubtasks(m.selectedTask.ID)
	if len(subtasks) > 0 {
		for i, t := range subtasks {
			sid := t.ID
			if t.CustomID != "" {
				sid = t.CustomID
			}

			assignee := taskAssigneeDisplay(t, true)
			pts := "0"
			if t.Points != nil {
				pts = fmt.Sprintf("%v", *t.Points)
			}

			pStr := lipgloss.NewStyle().Foreground(ColorSubtext).Render("NONE")
			if t.Priority != nil {
				pColor := t.Priority.Color
				if pColor == "" {
					pColor = "#6e7681"
				}
				pStr = lipgloss.NewStyle().Foreground(lipgloss.Color(pColor)).Bold(true).Render(strings.ToUpper(t.Priority.Priority))
			}

			status := t.Status.Status
			switch strings.ToLower(status) {
			case "todo", "open":
				status = StatusTodoStyle.Render(status)
			case "in progress", "active":
				status = StatusInProgressStyle.Render(status)
			case "done", "complete", "closed":
				status = StatusDoneStyle.Render(status)
			}

			b.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, sid, t.Name))
			b.WriteString(fmt.Sprintf("   %s | %s | PTS: %s | PRI: %s\n", status, assignee, pts, pStr))
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No subtasks."))
	}
	b.WriteString("\n")

	b.WriteString(divider + "\n\n")
	b.WriteString(SectionHeaderStyle.Render("CHECKLISTS") + "\n")
	if len(m.selectedTask.Checklists) > 0 {
		for i, cl := range m.selectedTask.Checklists {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, cl.Name))
			if len(cl.Items) == 0 {
				b.WriteString("   " + lipgloss.NewStyle().Foreground(ColorSubtext).Render("No items.") + "\n")
				continue
			}
			for j, item := range cl.Items {
				marker := "[ ]"
				if item.Resolved {
					marker = "[x]"
				}
				b.WriteString(fmt.Sprintf("   %d.%d %s %s\n", i+1, j+1, marker, item.Name))
			}
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No checklists."))
	}
	b.WriteString("\n\n")

	b.WriteString(divider + "\n\n")
	b.WriteString(SectionHeaderStyle.Render("ATTACHMENTS") + "\n")
	if len(m.selectedTask.Attachments) > 0 {
		for i, a := range m.selectedTask.Attachments {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, a.Title))
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No attachments."))
	}
	b.WriteString("\n\n")

	b.WriteString(divider + "\n\n")
	b.WriteString(SectionHeaderStyle.Render("COMMENTS") + "\n")
	if len(m.selectedComments) > 0 {
		commentWidth := m.width - 18
		if commentWidth < 24 {
			commentWidth = 24
		}

		for i, c := range m.selectedComments {
			authorName := c.User.Username
			if authorName == "" {
				authorName = "Unknown"
			}

			header := lipgloss.JoinHorizontal(
				lipgloss.Top,
				lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(fmt.Sprintf("ID: %d", i+1)),
				" ",
				lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(authorName),
				" ",
				lipgloss.NewStyle().Foreground(ColorSubtext).Render(formatClickUpTimestamp(c.Date)),
			)

			message := strings.TrimSpace(c.CommentText)
			if message == "" {
				message = lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("No comment text.")
			}

			cardDivider := lipgloss.NewStyle().
				Foreground(ColorBorder).
				Render(strings.Repeat("─", max(8, commentWidth-4)))

			commentStyle := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1).
				Width(commentWidth)

			if c.Parent != nil && *c.Parent != "" {
				commentStyle = commentStyle.MarginLeft(2)
			}

			b.WriteString(commentStyle.Render(header + "\n" + cardDivider + "\n" + message))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No comments."))
	}
	b.WriteString("\n\n")

	m.vp.SetContent(b.String())
}

func (m *AppModel) updateHelpContent() {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("ClickUp TUI - Help & Commands"))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Navigation"))
	b.WriteString("\n")
	b.WriteString("• Up/Down/j/k  : Navigate lists and text\n")
	b.WriteString("• Enter/Right  : Select item or view details\n")
	b.WriteString("• Esc/Left     : Go back to previous screen\n")
	b.WriteString("• 1-9          : While viewing a task, jump directly to a subtask by its number\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Global Commands (Type / to open prompt)"))
	b.WriteString("\n")
	b.WriteString("• /filter <text>           : Fuzzy search the current view\n")
	b.WriteString("• /filter assignee <name>  : Filter tasks by a specific assignee\n")
	b.WriteString("• /filter status <status>  : Filter tasks by a specific status\n")
	b.WriteString("• /filter id <id>          : Filter tasks by exact ID\n")
	b.WriteString("• /ticket <id>             : Jump directly to a ticket from anywhere (e.g. /ticket OMNI-123)\n")
	b.WriteString("• /search <text>           : Search tickets across the workspace\n")
	b.WriteString("• /search status:<status> assignee:<name> <text> : Search with filters\n")
	b.WriteString("• /space create <name>     : Create a new Space in the current Workspace\n")
	b.WriteString("• /space rename <name>     : Rename the highlighted Space in the spaces view\n")
	b.WriteString("• /list create <name>      : Create a new List in the current Folder or Space\n")
	b.WriteString("• /list rename <name>      : Rename the highlighted List in the list view\n")
	b.WriteString("• /list delete             : Delete the highlighted List after confirmation\n")
	b.WriteString("• /profile list            : Show available profiles\n")
	b.WriteString("• /profile create <name>   : Create a new empty profile and switch to it\n")
	b.WriteString("• /profile switch <name>   : Switch to a saved profile\n")
	b.WriteString("• /profile delete <name>   : Delete a profile with a yes/no confirmation prompt\n")
	b.WriteString("• /profile save <name>     : Save current settings as a profile and switch to it\n")
	b.WriteString("• /profile token <key>     : Set the API key for the active profile\n")
	b.WriteString("• /clear                   : Clear active filters\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Task Actions (when viewing a task)"))
	b.WriteString("\n")
	b.WriteString("• /status <status> : Change the ticket's status\n")
	b.WriteString("• /points <number> : Set story points\n")
	b.WriteString("• /share           : Copy ticket URL to clipboard\n")
	b.WriteString("• /delete          : Delete this ticket permanently\n")
	b.WriteString("• /move            : Move this ticket to another list\n")
	b.WriteString("• /assign <user>   : Assign the ticket to a user\n")
	b.WriteString("• /title           : Edit the ticket title\n")
	b.WriteString("• /desc            : Edit description (inline)\n")
	b.WriteString("• /copydesc        : Copy ticket description to clipboard\n")
	b.WriteString("• /editext         : Edit description in external $EDITOR\n")
	b.WriteString("• /subtask         : Create a new subtask\n")
	b.WriteString("• /checklist add <name>                    : Create a checklist\n")
	b.WriteString("• /checklist rename <checklist> <name>     : Rename a checklist by number\n")
	b.WriteString("• /checklist delete <checklist>            : Delete a checklist by number\n")
	b.WriteString("• /checklist item add <checklist> <name>   : Add an item to a checklist\n")
	b.WriteString("• /checklist item rename <checklist> <item> <name> : Rename a checklist item\n")
	b.WriteString("• /checklist item toggle <checklist> <item>        : Toggle checklist item state\n")
	b.WriteString("• /checklist item delete <checklist> <item>        : Delete a checklist item\n")
	b.WriteString("• /attach open <n>     : Open attachment preview in browser\n")
	b.WriteString("• /attach download <n> : Trigger attachment download\n")
	b.WriteString("• /attach share <n>    : Copy attachment URL\n")
	b.WriteString("• /attach upload        : Open a file browser to upload an attachment\n")
	b.WriteString("• c                : Add a comment\n")
	b.WriteString("• T                : Edit title\n")
	b.WriteString("• e                : Edit description (inline)\n")
	b.WriteString("• E                : Edit description in external $EDITOR\n")
	b.WriteString("• D                : Copy ticket description\n")
	b.WriteString("• t                : Create a new subtask\n")
	b.WriteString("• s                : Copy ticket URL to clipboard\n")
	b.WriteString("• r                : Refresh current view from API\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("List Actions"))
	b.WriteString("\n")
	b.WriteString("• a / n            : Create a new task (in Tasks view)\n")
	b.WriteString("• r                : Refresh the current list\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Default Routing Commands"))
	b.WriteString("\n")
	b.WriteString("• /default set         : Save currently highlighted Workspace/Space/List\n")
	b.WriteString("• /default clear       : Clear automatic startup routing\n")
	b.WriteString("• /default user <name> : Set base assignee filter\n")
	b.WriteString("• /default user clear  : Remove base assignee filter\n\n")

	help := "Use Up/Down to scroll | Press q or esc to close"
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render(help))
	m.vp.SetContent(b.String())
}

func (m *AppModel) renderHeader() string {
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4D4D"))
	whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00C853"))

	logo := redStyle.Render("╺") + whiteStyle.Render("❯") + greenStyle.Render("╸")

	ascii := ` ______     __         __     ______     __  __     __  __     ______  
/\  ___\   /\ \       /\ \   /\  ___\   /\ \/ /    /\ \/\ \   /\  == \ 
\ \ \____  \ \ \____  \ \ \  \ \ \____  \ \  _"-.  \ \ \_\ \  \ \  _-/ 
 \ \_____\  \ \_____\  \ \_\  \ \_____\  \ \_\ \_\  \ \_____\  \ \_\   
  \/_____/   \/_____/   \/_/   \/_____/   \/_/\/_/   \/_____/   \/_/`

	banner := HeaderBannerStyle.Foreground(ColorPrimary).Render(ascii)
	version := lipgloss.NewStyle().Foreground(ColorSubtext).Render("v1.2.0")

	infoStyle := lipgloss.NewStyle().Foreground(ColorText)
	userLine := infoStyle.Render("Signed in as: ") + ColorSecondaryStyle.Render(m.currentUser)
	profileLine := infoStyle.Render("Profile: ") + ColorSecondaryStyle.Render(m.activeProfile)

	workspace := "None"
	if m.selectedTeam != "" {
		for _, t := range m.allTeams {
			if t.ID == m.selectedTeam {
				workspace = t.Name
				break
			}
		}
	}
	workspaceLine := infoStyle.Render("Workspace: ") + ColorSecondaryStyle.Render(workspace)

	headerInfo := lipgloss.JoinVertical(
		lipgloss.Left,
		logo+"  "+version,
		"",
		profileLine,
		userLine,
		workspaceLine,
	)

	return HeaderInsetStyle.Render(lipgloss.JoinVertical(lipgloss.Left, banner, "", headerInfo))
}

func (m *AppModel) selectedTeamName() string {
	for _, t := range m.allTeams {
		if t.ID == m.selectedTeam {
			return t.Name
		}
	}
	return ""
}

func (m *AppModel) selectedSpaceName() string {
	for _, s := range m.allSpaces {
		if s.ID == m.selectedSpace {
			return s.Name
		}
	}
	return ""
}

func (m *AppModel) selectedListName() string {
	if m.selectedFolder != nil {
		for _, l := range m.selectedFolder.Lists {
			if l.ID == m.selectedList {
				return l.Name
			}
		}
	}
	for _, l := range m.allLists {
		if l.ID == m.selectedList {
			return l.Name
		}
	}
	for _, f := range m.allFolders {
		for _, l := range f.Lists {
			if l.ID == m.selectedList {
				return l.Name
			}
		}
	}
	return ""
}

func (m *AppModel) breadcrumb() string {
	var parts []string

	switch m.state {
	case stateTeams:
		parts = append(parts, "Workspaces")
	case stateSpaces:
		parts = append(parts, "Workspaces")
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		parts = append(parts, "Spaces")
	case stateLists:
		parts = append(parts, "Workspaces")
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		if name := m.selectedSpaceName(); name != "" {
			parts = append(parts, "Space: "+name)
		}
		if m.selectedFolder != nil && m.selectedFolder.Name != "" {
			parts = append(parts, "Folder: "+m.selectedFolder.Name)
		} else {
			parts = append(parts, "Lists")
		}
	case stateTasks, stateSearchResults, stateTaskDetail, stateComment, stateEditTitle, stateEditDesc, stateCreateSubtask, stateEditComment:
		parts = append(parts, "Workspaces")
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		if name := m.selectedSpaceName(); name != "" {
			parts = append(parts, "Space: "+name)
		}
		if m.selectedFolder != nil && m.selectedFolder.Name != "" {
			parts = append(parts, "Folder: "+m.selectedFolder.Name)
		}
		if name := m.selectedListName(); name != "" {
			parts = append(parts, "List: "+name)
		}
		if m.state == stateTasks {
			parts = append(parts, "Tasks")
		}
		if m.state == stateSearchResults {
			label := "Search"
			if q := strings.TrimSpace(m.searchQuery); q != "" {
				label = fmt.Sprintf("Search: %q", q)
			}
			parts = append(parts, label)
		}
		if m.state == stateTaskDetail || m.state == stateComment || m.state == stateEditTitle || m.state == stateEditDesc || m.state == stateCreateSubtask || m.state == stateEditComment {
			if m.selectedTask.Name != "" {
				taskID := m.selectedTask.ID
				if m.selectedTask.CustomID != "" {
					taskID = m.selectedTask.CustomID
				}
				parts = append(parts, fmt.Sprintf("Task: [%s] %s", taskID, m.selectedTask.Name))
			}
		}
	case stateHelp:
		parts = append(parts, "Help")
	case stateCreateTask:
		parts = append(parts, "Workspaces")
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		if name := m.selectedSpaceName(); name != "" {
			parts = append(parts, "Space: "+name)
		}
		if m.selectedFolder != nil && m.selectedFolder.Name != "" {
			parts = append(parts, "Folder: "+m.selectedFolder.Name)
		}
		if name := m.selectedListName(); name != "" {
			parts = append(parts, "List: "+name)
		}
		parts = append(parts, "New Task")
	case stateMovePicker:
		parts = append(parts, "Workspaces")
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		if name := m.selectedSpaceName(); name != "" {
			parts = append(parts, "Space: "+name)
		}
		if name := m.selectedListName(); name != "" {
			parts = append(parts, "List: "+name)
		}
		parts = append(parts, "Move Ticket")
	case stateCommand:
		return ""
	case stateConfirmProfileDelete:
		parts = append(parts, "Profiles", "Delete Profile")
	case stateConfirmListDelete:
		parts = append(parts, "Lists", "Delete List")
	case stateConfirmDiscardDesc:
		parts = append(parts, "Task", "Discard Description Changes")
	case stateFilePicker:
		if name := m.selectedTeamName(); name != "" {
			parts = append(parts, "Workspace: "+name)
		}
		if name := m.selectedSpaceName(); name != "" {
			parts = append(parts, "Space: "+name)
		}
		if name := m.selectedListName(); name != "" {
			parts = append(parts, "List: "+name)
		}
		if m.selectedTask.Name != "" {
			taskID := m.selectedTask.ID
			if m.selectedTask.CustomID != "" {
				taskID = m.selectedTask.CustomID
			}
			parts = append(parts, fmt.Sprintf("Task: [%s] %s", taskID, m.selectedTask.Name))
		}
		parts = append(parts, "Upload Attachment")
	}

	if len(parts) == 0 {
		return ""
	}

	return HeaderInsetStyle.Render(BreadcrumbStyle.Render(strings.Join(parts, "  >  ")))
}

func (m *AppModel) View() string {
	if m.width == 0 {
		return "Starting..."
	}

	if m.loading {
		loadingBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 4).Render(fmt.Sprintf("%s %s", m.spinner.View(), m.loadingMsg))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, loadingBox)
	}

	var mainContent string

	switch m.state {
	case stateTeams, stateSpaces, stateLists, stateTasks, stateSearchResults, stateFilePicker:
		view := m.activeList.View()
		if m.state == stateTasks {
			// Integrate into the same row as the list help, but trim the justified whitespace
			lines := strings.Split(view, "\n")
			lastIdx := len(lines) - 1
			for lastIdx >= 0 && strings.TrimSpace(lines[lastIdx]) == "" {
				lastIdx--
			}
			if lastIdx >= 0 {
				style := lipgloss.NewStyle().Foreground(ColorSubtext)
				lines[lastIdx] = strings.TrimRight(lines[lastIdx], " ") + style.Render(" • a/n: new task • r: refresh")
				view = strings.Join(lines, "\n")
			}
		} else if m.state == stateSearchResults {
			lines := strings.Split(view, "\n")
			lastIdx := len(lines) - 1
			for lastIdx >= 0 && strings.TrimSpace(lines[lastIdx]) == "" {
				lastIdx--
			}
			if lastIdx >= 0 {
				style := lipgloss.NewStyle().Foreground(ColorSubtext)
				lines[lastIdx] = strings.TrimRight(lines[lastIdx], " ") + style.Render(" • enter: open ticket • esc: back")
				view = strings.Join(lines, "\n")
			}
		} else if m.state == stateFilePicker {
			lines := strings.Split(view, "\n")
			lastIdx := len(lines) - 1
			for lastIdx >= 0 && strings.TrimSpace(lines[lastIdx]) == "" {
				lastIdx--
			}
			if lastIdx >= 0 {
				style := lipgloss.NewStyle().Foreground(ColorSubtext)
				lines[lastIdx] = strings.TrimRight(lines[lastIdx], " ") + style.Render(" • enter: open folder/upload file • .: hidden • esc: cancel")
				view = strings.Join(lines, "\n")
			}
		}
		mainContent = view
	case stateTaskDetail:
		hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("q: back • a/n: new task • c: comment • T: edit title • e: edit desc • E: vim edit • D: copy desc • t: subtask • o: open • s: copy • r: refresh")
		mainContent = m.vp.View() + "\n" + hint
	case stateHelp:
		mainContent = m.vp.View()
	case stateComment:
		header := TitleStyle.Render("Adding Comment:")
		if m.replyToUser != "" {
			header = TitleStyle.Render(fmt.Sprintf("Replying to %s:", m.replyToUser))
		}
		mainContent = m.vp.View() + "\n\n" + header + "\n" + m.commentInput.View() + "\n(Ctrl+S to submit, Ctrl+E for Vim, Esc to cancel)"
	case stateCreateTask:
		mainContent = m.activeList.View() + "\n\n" + lipgloss.NewStyle().Bold(true).Render("New Task: ") + m.taskInput.View() + "\n" + lipgloss.NewStyle().Foreground(ColorSubtext).Render("Enter to create | Esc to cancel")
	case stateCreateSubtask:
		header := TitleStyle.Render(fmt.Sprintf("New Subtask of: %s", m.selectedTask.Name))
		hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("Enter to create | Esc to cancel")
		mainContent = header + "\n\n" + lipgloss.NewStyle().Bold(true).Render("Subtask name: ") + m.taskInput.View() + "\n" + hint
	case stateEditTitle:
		mainContent = TitleStyle.Render("Editing Title:") + "\n\n" + lipgloss.NewStyle().Bold(true).Render("Title: ") + m.taskInput.View() + "\n" + lipgloss.NewStyle().Foreground(ColorSubtext).Render("Enter to save | Esc to cancel")
	case stateMovePicker:
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("Move Ticket To List") + "\n\n")
		for i, l := range m.moveCandidateLists {
			if i == m.suggestIdx {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("> " + l.Name))
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorText).Render("  " + l.Name))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(ColorSubtext).Render("Up/Down to select | Enter to move | Esc to cancel"))
		mainContent = sb.String()
	case stateEditDesc:
		mainContent = m.renderEditDesc()
	case stateEditComment:
		mainContent = m.vp.View() + "\n\n" + TitleStyle.Render("Editing Comment:") + "\n" + m.commentInput.View() + "\n(Ctrl+S to save, Ctrl+E for Vim, Esc to cancel)"
	case stateConfirmProfileDelete:
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Delete Profile?") + "\n\n" +
					fmt.Sprintf("Delete profile %q?", m.pendingDeleteProfile) + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: yes • n/esc: no"),
			)
		mainContent = lipgloss.Place(m.width, m.height-8, lipgloss.Center, lipgloss.Center, box)
	case stateConfirmListDelete:
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Delete List?") + "\n\n" +
					fmt.Sprintf("Delete list %q?", m.pendingDeleteListName) + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: yes • n/esc: no"),
			)
		mainContent = lipgloss.Place(m.width, m.height-8, lipgloss.Center, lipgloss.Center, box)
	case stateConfirmDiscardDesc:
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Discard Description Changes?") + "\n\n" +
					"Your unsaved description edits will be lost." + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: discard • n/esc: keep editing"),
			)
		mainContent = lipgloss.Place(m.width, m.height-8, lipgloss.Center, lipgloss.Center, box)
	case stateCommand:
		if m.prevState == stateTaskDetail || m.prevState == stateHelp {
			mainContent = m.vp.View()
		} else {
			mainContent = m.activeList.View()
		}
	}

	var sb strings.Builder
	if m.state == stateCommand && len(m.filteredSuggest) > 0 {
		sb.WriteString("\n")
		for i, s := range m.filteredSuggest {
			textStyle := lipgloss.NewStyle().Width(35).Foreground(ColorPrimary).Bold(i == m.suggestIdx)
			descStyle := lipgloss.NewStyle().Foreground(ColorText).PaddingLeft(2)

			if i == m.suggestIdx {
				textStyle = textStyle.Foreground(ColorSecondary)
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorSecondary).Render("> "))
			} else {
				sb.WriteString("  ")
			}

			sb.WriteString(textStyle.Render(s.Text))
			sb.WriteString(descStyle.Render(s.Desc))

			if i < len(m.filteredSuggest)-1 {
				sb.WriteString("\n")
			}
		}
	}

	bottomBar := m.cmdInput.View() + sb.String()
	if m.state != stateCommand {
		bottomBar = lipgloss.NewStyle().Foreground(ColorSubtext).Render("Type / to enter command")
	}

	if breadcrumb := m.breadcrumb(); breadcrumb != "" {
		mainContent = breadcrumb + "\n\n" + mainContent
	}

	if m.popupMsg != "" {
		icon := "✓ "
		color := ColorSecondary
		if strings.HasPrefix(m.popupMsg, "Error") {
			icon = "✖ "
			color = ColorError
		}

		popupBox := lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(color).
			Bold(true).
			Padding(0, 1).
			Render(icon + m.popupMsg)

		shadow := lipgloss.NewStyle().Foreground(lipgloss.Color("234")).Render("▌")
		popupStr := popupBox + shadow

		space := m.width - lipgloss.Width(bottomBar) - lipgloss.Width(popupStr) - 2
		if space > 0 {
			bottomBar = bottomBar + strings.Repeat(" ", space) + popupStr
		} else {
			bottomBar = bottomBar + " " + popupStr
		}
	}

	fullUI := m.renderHeader() + "\n" + mainContent + "\n\n" + bottomBar
	return BaseStyle.Render(fullUI)
}

func flattenComments(comments []clickup.Comment) []clickup.Comment {
	var flat []clickup.Comment
	for _, c := range comments {
		flat = append(flat, c)
		if len(c.Replies) > 0 {
			// Ensure parent ID is set for children to trigger indentation logic
			for i := range c.Replies {
				if c.Replies[i].Parent == nil || *c.Replies[i].Parent == "" {
					pID := c.ID
					c.Replies[i].Parent = &pID
				}
			}
			flat = append(flat, flattenComments(c.Replies)...)
		}
	}
	return flat
}
