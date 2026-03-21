package ui

import (
	"fmt"
	"os"
	"os/exec"
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
	stateTaskDetail
	stateComment
	stateCommand
	stateHelp
	stateCreateTask
	stateMovePicker
	stateEditDesc
	stateCreateSubtask
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

type taskItem clickup.Task
func (t taskItem) Title() string {
	id := t.ID
	if t.CustomID != "" {
		id = t.CustomID
	}
	return fmt.Sprintf("[%s] %s", id, t.Name) 
}
func (t taskItem) Description() string { 
	assignee := "unassigned"
	if len(t.Assignees) > 0 { assignee = strings.ToLower(t.Assignees[0].Username) }
	pts := "0"
	if t.Points != nil { pts = fmt.Sprintf("%v", *t.Points) }
	return fmt.Sprintf("Status: %s | Assignee: %s | PTS: %s", t.Status.Status, assignee, pts) 
}
func (t taskItem) FilterValue() string { 
	assignee := "unassigned"
	if len(t.Assignees) > 0 { assignee = strings.ToLower(t.Assignees[0].Username) }
	
	title := strings.ToLower(t.Name)
	status := strings.ToLower(t.Status.Status)
	
	id := t.ID
	if t.CustomID != "" {
		id = t.CustomID
	}
	idLower := strings.ToLower(id)
	
	return fmt.Sprintf("id:%s assignee:%s status:%s title:%s %s %s", idLower, assignee, status, title, t.Name, idLower)
}

type Suggestion struct {
	Text string
	Desc string
}

type AppModel struct {
	state        state
	prevState    state
	cfg          *config.Config
	client       *clickup.Client
	
	activeList   *list.Model

	teamsList    list.Model
	spacesList   list.Model
	listsList    list.Model
	tasksList    list.Model
	
	allTeams     []clickup.Team
	allSpaces    []clickup.Space
	allLists     []clickup.List
	allTasks     []clickup.Task

	commentInput textinput.Model
	taskInput    textinput.Model // used for create / rename
	descInput    textarea.Model
	cmdInput     textinput.Model
	vp           viewport.Model
	width        int
	height       int
	
	suggestions     []Suggestion
	filteredSuggest []Suggestion
	suggestIdx      int
	
	selectedTeam  string
	selectedSpace string
	selectedList  string
	selectedTask       clickup.Task
	taskHistory        []clickup.Task
	moveCandidateLists []clickup.List
	moveTaskID         string
	teamMembers        []clickup.Member
	parentTaskID       string // used when creating subtasks
	
	loading    bool
	loadingMsg string
	spinner    spinner.Model
	popupMsg   string
	
	currentUser string
	err error
}

type teamsMsg []clickup.Team
type spacesMsg []clickup.Space
type listsMsg []clickup.List
type tasksMsg []clickup.Task
type taskDetailMsg *clickup.Task
type errMsg error
type clearPopupMsg struct{}
type taskCreatedMsg clickup.Task
type moveListsReadyMsg []clickup.List
type teamMembersMsg []clickup.Member
type editorFinishedMsg struct{ content string; err error }

func fetchTeamsCmd(c *clickup.Client) tea.Cmd {
	return func() tea.Msg {
		teams, err := c.GetTeams()
		if err != nil { return errMsg(err) }
		return teamsMsg(teams)
	}
}

func fetchSpacesCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		spaces, err := c.GetSpaces(teamID)
		if err != nil { return errMsg(err) }
		return spacesMsg(spaces)
	}
}

func fetchListsCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		lists, err := c.GetSpaceLists(spaceID)
		if err != nil { return errMsg(err) }
		return listsMsg(lists)
	}
}

func fetchTasksCmd(c *clickup.Client, listID string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := c.GetTasks(listID)
		if err != nil { return errMsg(err) }
		return tasksMsg(tasks)
	}
}

func fetchTaskCmd(c *clickup.Client, taskID, teamID string) tea.Cmd {
	return func() tea.Msg {
		task, err := c.GetTask(taskID, teamID)
		if err != nil { return errMsg(err) }
		return taskDetailMsg(task)
	}
}

func createTaskCmd(c *clickup.Client, listID, name string) tea.Cmd {
	return func() tea.Msg {
		task, err := c.CreateTask(listID, name)
		if err != nil { return errMsg(err) }
		return taskCreatedMsg(*task)
	}
}

func createSubtaskCmd(c *clickup.Client, listID, parentID, name string) tea.Cmd {
	return func() tea.Msg {
		task, err := c.CreateSubtask(listID, parentID, name)
		if err != nil { return errMsg(err) }
		return taskCreatedMsg(*task)
	}
}

func fetchAllListsForMoveCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		lists, err := c.GetSpaceLists(spaceID)
		if err != nil { return errMsg(err) }
		return moveListsReadyMsg(lists)
	}
}

func fetchTeamMembersCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		members, err := c.GetTeamMembers(teamID)
		if err != nil { return errMsg(err) }
		return teamMembersMsg(members)
	}
}

func openExternalEditorCmd(initialContent string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" { editor = "vim" }
	
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

	c := clickup.NewClient(cfg.ClickupAPIKey)

	var currentUser string = "Unauthenticated"
	// Fetch user identity if missing from config
	if cfg.ClickupAPIKey != "NO_TOKEN" {
		u, err := c.GetUser()
		if err == nil {
			currentUser = u.Username
			if cfg.ClickupUserName == "" {
				cfg.ClickupUserName = u.Username
				config.SaveConfig(cfg)
			}
		}
	}

	var allTeams []clickup.Team
	var items []list.Item
	teams, err := c.GetTeams()
	if err == nil {
		allTeams = teams
		for _, t := range teams {
			items = append(items, teamItem(t))
		}
	}

	teamsList := list.New(items, list.NewDefaultDelegate(), 0, 0)
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

	ci := textinput.New()
	ci.Placeholder = "Type a comment..."
	ci.CharLimit = 156
	ci.Width = 40

	cmd := textinput.New()
	cmd.Placeholder = "Enter slash command (e.g. /filter) or /help..."
	cmd.Prompt = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("> ")
	cmd.CharLimit = 156
	cmd.Width = 50

	// create a separate textinput for task creation
	ti := textinput.New()
	ti.Placeholder = "Task name..."
	ti.CharLimit = 200

	// textarea for description editing
	da := textarea.New()
	da.Placeholder = "Enter description..."
	da.CharLimit = 5000
	da.SetWidth(80)
	da.SetHeight(10)

	vp := viewport.New(0, 0)
	vp.Style = DetailStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	m := &AppModel{
		state:        stateTeams,
		prevState:    stateTeams,
		cfg:          cfg,
		client:       c,
		teamsList:    teamsList,
		spacesList:   spacesList,
		listsList:    listsList,
		tasksList:    tasksList,
		allTeams:     allTeams,
		commentInput: ci,
		taskInput:    ti,
		descInput:    da,
		cmdInput:     cmd,
		vp:           vp,
		spinner:      s,
		currentUser:  currentUser,
	}
	m.activeList = &m.teamsList
	
	// Hydrate deep trees if configs are present
	if cfg.ClickupTeamID != "" {
		m.selectedTeam = cfg.ClickupTeamID
		spaces, err := c.GetSpaces(cfg.ClickupTeamID)
		if err == nil {
			m.allSpaces = spaces
			var sItems []list.Item
			for _, s := range spaces { sItems = append(sItems, spaceItem(s)) }
			m.spacesList.SetItems(sItems)
			m.state = stateSpaces
			m.activeList = &m.spacesList
			
			if cfg.ClickupSpaceID != "" {
				m.selectedSpace = cfg.ClickupSpaceID
				lists, err := c.GetSpaceLists(cfg.ClickupSpaceID)
				if err == nil {
					m.allLists = lists
					var lItems []list.Item
					for _, l := range lists { lItems = append(lItems, listItem(l)) }
					m.listsList.SetItems(lItems)
					m.state = stateLists
					m.activeList = &m.listsList
					
					if cfg.ClickupListID != "" {
						m.selectedList = cfg.ClickupListID
						tasks, err := c.GetTasks(cfg.ClickupListID)
						if err == nil {
							m.allTasks = tasks
							m.applyTaskFilter("")
							m.state = stateTasks
							m.activeList = &m.tasksList
						}
					}
				}
			}
		}
	}
	
	return m
}

func (m *AppModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *AppModel) updateLayout() {
	h, v := BaseStyle.GetFrameSize()
	
	// Header is ~11-12 lines now with ASCII art
	contentH := m.height - v - 2 - 12 
	
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
	m.vp.Width = m.width - h
	m.vp.Height = contentH
	m.descInput.SetWidth(m.width - h)
	m.descInput.SetHeight(contentH)
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
		if (s == "/" || s == ":") && m.state != stateCommand && m.state != stateComment {
			m.prevState = m.state
			m.state = stateCommand
			m.cmdInput.Focus()
			m.cmdInput.SetValue(s)
			m.cmdInput.SetCursor(len(m.cmdInput.Value()))
			m.updateCommandSuggestions()
			m.updateLayout()
			// Lazily fetch team members for /assign suggestions
			if m.state == stateCommand && m.prevState == stateTaskDetail && len(m.teamMembers) == 0 && m.selectedTeam != "" {
				return m, tea.Batch(textinput.Blink, fetchTeamMembersCmd(m.client, m.selectedTeam))
			}
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
		for _, s := range msg { items = append(items, spaceItem(s)) }
		m.spacesList.SetItems(items)
		m.state = stateSpaces
		m.activeList = &m.spacesList
		return m, nil
	case listsMsg:
		m.loading = false
		m.allLists = msg
		var items []list.Item
		for _, l := range msg { items = append(items, listItem(l)) }
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
	case taskDetailMsg:
		m.loading = false
		m.selectedTask = *msg
		m.taskHistory = nil
		m.state = stateTaskDetail
		m.updateViewportContent()
		return m, nil
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
		m.moveCandidateLists = []clickup.List(msg)
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
	case editorFinishedMsg:
		if msg.err == nil && msg.content != m.selectedTask.Desc {
			desc := strings.TrimRight(msg.content, "\n")
			if err := m.client.UpdateDescription(m.selectedTask.ID, desc); err == nil {
				m.selectedTask.Desc = desc
				for i, t := range m.allTasks {
					if t.ID == m.selectedTask.ID {
						m.allTasks[i].Desc = desc
						break
					}
				}
			}
		}
		m.state = stateTaskDetail
		m.updateViewportContent()
		return m, nil
	case errMsg:
		m.loading = false
		m.err = msg
		return m, nil
	case clearPopupMsg:
		m.popupMsg = ""
		return m, nil
	}
	
	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.state {
	case stateTeams, stateSpaces, stateLists, stateTasks:
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
	case stateEditDesc:
		return m.updateEditDesc(msg)
	case stateCreateSubtask:
		return m.updateCreateSubtask(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) getSubtasks(parentID string) []clickup.Task {
	var res []clickup.Task
	for _, t := range m.allTasks {
		if t.Parent != nil {
			parentStr, ok := t.Parent.(string)
			if ok && parentStr == parentID {
				res = append(res, t)
			}
		}
	}
	return res
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
			if len(t.Assignees) > 0 {
				assignees[strings.ToLower(t.Assignees[0].Username)] = true
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
		sugs = append(sugs, Suggestion{"/desc", "Edit the ticket description (inline)"})
		sugs = append(sugs, Suggestion{"/editext", "Edit description in $EDITOR (vim etc)"})
		sugs = append(sugs, Suggestion{"/subtask", "Add a subtask to this ticket"})

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
		
		statuses := make(map[string]bool)
		for _, t := range m.allTasks {
			statuses[strings.ToLower(t.Status.Status)] = true
		}
		for s := range statuses {
			sugs = append(sugs, Suggestion{"/status " + s, "Set status to " + s})
		}
	} else if m.prevState == stateTeams {
		sugs = append(sugs, Suggestion{"/filter", "Filter workspaces by name"})
		for _, t := range m.allTeams {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find workspace " + t.Name})
		}
	} else if m.prevState == stateSpaces {
		sugs = append(sugs, Suggestion{"/filter", "Filter spaces by name"})
		for _, t := range m.allSpaces {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find space " + t.Name})
		}
	} else if m.prevState == stateLists {
		sugs = append(sugs, Suggestion{"/filter", "Filter lists by name"})
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
		if t.Parent != nil {
			continue // Hide subtasks from the main root list view
		}
		
		assignee := "unassigned"
		if len(t.Assignees) > 0 { assignee = strings.ToLower(t.Assignees[0].Username) }
		
		// If a default user is set, heavily prioritize it unless an explicit assignee override is typed
		if defaultUser != "" && !strings.HasPrefix(query, "assignee ") && defaultUser != "clear" {
			if !strings.Contains(assignee, defaultUser) && !fuzzyMatch(defaultUser, assignee) {
				continue
			}
		}

		if query == "" {
			items = append(items, taskItem(t))
			if t.Points != nil { totalPoints += *t.Points }
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
			if fuzzyMatch(term, assignee) {
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
			if fuzzyMatch(query, title) || fuzzyMatch(query, assignee) || fuzzyMatch(query, status) || fuzzyMatch(query, idLower) {
				items = append(items, taskItem(t))
				if t.Points != nil { totalPoints += *t.Points }
			}
		}
	}
	
	m.tasksList.Title = fmt.Sprintf("Tasks (Total Points: %v)", totalPoints)
	m.tasksList.SetItems(items)
}

func (m *AppModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			if m.state == stateTasks {
				m.state = stateLists
				m.activeList = &m.listsList
			} else if m.state == stateLists {
				m.state = stateSpaces
				m.activeList = &m.spacesList
			} else if m.state == stateSpaces {
				m.state = stateTeams
				m.activeList = &m.teamsList
			}
			return m, nil
		case "a", "n":
			if m.state == stateTasks {
				m.state = stateCreateTask
				m.taskInput.SetValue("")
				m.taskInput.Focus()
				return m, textinput.Blink
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
				return m, tea.Batch(m.spinner.Tick, fetchTasksCmd(m.client, m.selectedList))
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
				if i, ok := m.activeList.SelectedItem().(listItem); ok {
					m.selectedList = i.ID
					m.loading = true
					m.loadingMsg = "Loading tasks..."
					return m, tea.Batch(m.spinner.Tick, fetchTasksCmd(m.client, m.selectedList))
				}
			case stateTasks:
				if i, ok := m.activeList.SelectedItem().(taskItem); ok {
					m.selectedTask = clickup.Task(i)
					m.taskHistory = nil
					m.state = stateTaskDetail
					m.updateViewportContent()
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
				m.state = stateTasks
			}
			return m, nil
		case "c":
			m.state = stateComment
			m.commentInput.Focus()
			return m, textinput.Blink
		case "e":
			m.state = stateEditDesc
			m.descInput.SetValue(m.selectedTask.Desc)
			m.descInput.Focus()
			return m, textarea.Blink
		case "E":
			// Open in external editor
			return m, openExternalEditorCmd(m.selectedTask.Desc)
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
				m.selectedTask = subtasks[idx]
				m.updateViewportContent()
			}
			return m, nil
		case "r":
			m.loading = true
			m.loadingMsg = "Refreshing task..."
			return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam))
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *AppModel) updateComment(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			v := m.commentInput.Value()
			if v != "" {
				m.client.AddComment(m.selectedTask.ID, v)
				m.commentInput.SetValue("")
				m.commentInput.Blur()
				m.state = stateTaskDetail
			}
			return m, nil
		case tea.KeyEsc:
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
				return m, tea.Batch(m.spinner.Tick, createTaskCmd(m.client, m.selectedList, name))
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
				return m, tea.Batch(m.spinner.Tick, createSubtaskCmd(m.client, m.selectedList, m.parentTaskID, name))
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
			if m.suggestIdx > 0 { m.suggestIdx-- }
			return m, nil
		case "down", "j":
			if m.suggestIdx < len(m.moveCandidateLists)-1 { m.suggestIdx++ }
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

func (m *AppModel) updateEditDesc(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.descInput.Blur()
			m.state = stateTaskDetail
			return m, nil
		case tea.KeyCtrlS:
			desc := m.descInput.Value()
			m.descInput.Blur()
			m.state = stateTaskDetail
			if err := m.client.UpdateDescription(m.selectedTask.ID, desc); err == nil {
				m.selectedTask.Desc = desc
				for i, t := range m.allTasks {
					if t.ID == m.selectedTask.ID {
						m.allTasks[i].Desc = desc
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
			} else if strings.HasPrefix(val, "/status ") {
				if m.prevState == stateTaskDetail {
					newStatus := strings.TrimPrefix(val, "/status ")
					m.client.UpdateStatus(m.selectedTask.ID, newStatus)
					// Update local view
					m.selectedTask.Status.Status = newStatus
					m.updateViewportContent()
					// Update local cache
					for i, t := range m.allTasks {
						if t.ID == m.selectedTask.ID {
							m.allTasks[i].Status.Status = newStatus
							break
						}
					}
					// Re-apply filter so lists reflect changes
					m.applyHierarchyFilter(strings.TrimPrefix(m.cmdInput.Value(), "/filter "))
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
					return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, id, teamID))
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
							if t.ID != taskID { kept = append(kept, t) }
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
					m.descInput.SetValue(m.selectedTask.Desc)
					m.descInput.Focus()
					return m, textarea.Blink
				}
			} else if strings.HasPrefix(val, "/editext") {
				if m.prevState == stateTaskDetail {
					return m, openExternalEditorCmd(m.selectedTask.Desc)
				}
			} else if strings.HasPrefix(val, "/subtask") {
				if m.prevState == stateTaskDetail {
					m.parentTaskID = m.selectedTask.ID
					m.state = stateCreateSubtask
					m.taskInput.SetValue("")
					m.taskInput.Focus()
					return m, textinput.Blink
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
							if a.ID != addID { removes = append(removes, a.ID) }
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
						m.cfg.ClickupSpaceID = m.selectedSpace
						m.cfg.ClickupTeamID = m.selectedTeam
						config.SaveConfig(m.cfg)
					}
				}
				m.popupMsg = "Default set successfully"
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
	
	b.WriteString(TitleStyle.Render(fmt.Sprintf("[%s] %s", m.selectedTask.Status.Status, m.selectedTask.Name)) + "\n\n")
	
	assignee := "Unassigned"
	if len(m.selectedTask.Assignees) > 0 { assignee = m.selectedTask.Assignees[0].Username }
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Assignee: ") + assignee + "\n\n")
	
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Description:") + "\n")
	b.WriteString(m.selectedTask.Desc + "\n\n")
	
	pts := "0"
	if m.selectedTask.Points != nil { pts = fmt.Sprintf("%v", *m.selectedTask.Points) }
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Story Points: ") + pts + "\n\n")
	
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Subtasks:") + "\n")
	subtasks := m.getSubtasks(m.selectedTask.ID)
	if len(subtasks) > 0 {
		for i, t := range subtasks {
			id := t.ID
			if t.CustomID != "" {
				id = t.CustomID
			}
			b.WriteString(fmt.Sprintf("%d. [%s] %s (%s)\n", i+1, id, t.Name, t.Status.Status))
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No subtasks."))
	}
	b.WriteString("\n\n")

	b.WriteString("\n\n")
	help := lipgloss.NewStyle().Foreground(ColorSubtext).Render("q: back | c: comment | e: edit desc | E: vim edit | t: subtask | s: copy | r: refresh")
	b.WriteString(help)

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
	b.WriteString("• /clear                   : Clear active filters\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Task Actions (when viewing a task)"))
	b.WriteString("\n")
	b.WriteString("• /status <status> : Change the ticket's status\n")
	b.WriteString("• /points <number> : Set story points\n")
	b.WriteString("• /share           : Copy ticket URL to clipboard\n")
	b.WriteString("• /delete          : Delete this ticket permanently\n")
	b.WriteString("• /move            : Move this ticket to another list\n")
	b.WriteString("• /assign <user>   : Assign the ticket to a user\n")
	b.WriteString("• /desc            : Edit description (inline)\n")
	b.WriteString("• /editext         : Edit description in external $EDITOR\n")
	b.WriteString("• /subtask         : Create a new subtask\n")
	b.WriteString("• c                : Add a comment\n")
	b.WriteString("• e                : Edit description (inline)\n")
	b.WriteString("• E                : Edit description in external $EDITOR\n")
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

	ascii := `
     ______     __         __     ______     __  __     __  __     ______  
    /\  ___\   /\ \       /\ \   /\  ___\   /\ \/ /    /\ \/\ \   /\  == \ 
    \ \ \____  \ \ \____  \ \ \  \ \ \____  \ \  _"-.  \ \ \_\ \  \ \  _-/ 
     \ \_____\  \ \_____\  \ \_\  \ \_____\  \ \_\ \_\  \ \_____\  \ \_\   
      \/_____/   \/_____/   \/_/   \/_____/   \/_/\/_/   \/_____/   \/_/   
`
	
	banner := lipgloss.NewStyle().Foreground(ColorPrimary).Render(ascii)
	version := lipgloss.NewStyle().Foreground(ColorSubtext).Render("v1.2.0")
	
	infoStyle := lipgloss.NewStyle().Foreground(ColorText)
	userLine := infoStyle.Render("Signed in as: ") + ColorSecondaryStyle.Render(m.currentUser)
	
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

	headerInfo := fmt.Sprintf("\n  %s  %s\n\n  %s\n  %s", logo, version, userLine, workspaceLine)
	
	return lipgloss.JoinVertical(lipgloss.Left, banner, headerInfo)
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
	case stateTeams, stateSpaces, stateLists, stateTasks:
		mainContent = m.activeList.View()
	case stateTaskDetail:
		mainContent = m.vp.View()
	case stateHelp:
		mainContent = m.vp.View()
	case stateComment:
		mainContent = m.vp.View() + "\n\n" + m.commentInput.View() + "\n(Enter to submit, Esc to cancel)"
	case stateCreateTask:
		mainContent = m.activeList.View() + "\n\n" + lipgloss.NewStyle().Bold(true).Render("New Task: ") + m.taskInput.View() + "\n" + lipgloss.NewStyle().Foreground(ColorSubtext).Render("Enter to create | Esc to cancel")
	case stateCreateSubtask:
		header := TitleStyle.Render(fmt.Sprintf("New Subtask of: %s", m.selectedTask.Name))
		hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("Enter to create | Esc to cancel")
		mainContent = header + "\n\n" + lipgloss.NewStyle().Bold(true).Render("Subtask name: ") + m.taskInput.View() + "\n" + hint
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
		header := TitleStyle.Render(fmt.Sprintf("Editing: %s", m.selectedTask.Name))
		hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("Ctrl+S to save | Esc to cancel")
		mainContent = header + "\n\n" + m.descInput.View() + "\n" + hint
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

	if m.popupMsg != "" {
		popupBox := lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(ColorSecondary).
			Bold(true).
			Padding(0, 1).
			Render("✓ " + m.popupMsg)
			
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
