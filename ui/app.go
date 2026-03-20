package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
	selectedTask  clickup.Task
	taskHistory   []clickup.Task
	
	err error
}

func InitialModel() *AppModel {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		cfg = &config.Config{ClickupAPIKey: "NO_TOKEN"}
	}

	c := clickup.NewClient(cfg.ClickupAPIKey)

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

	vp := viewport.New(0, 0)
	vp.Style = DetailStyle

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
		cmdInput:     cmd,
		vp:           vp,
	}
	m.activeList = &m.teamsList
	return m
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

func (m *AppModel) updateLayout() {
	h, v := BaseStyle.GetFrameSize()
	
	contentH := m.height - v - 2 // reserved 2 lines for input/help bar
	
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
			return m, textinput.Blink
		}
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
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

func (m *AppModel) loadTasksAndSwitch(listID string) {
	m.selectedList = listID
	tasks, err := m.client.GetTasks(m.selectedList)
	if err == nil {
		m.allTasks = tasks
		m.taskHistory = nil
		m.applyTaskFilter("") 
		m.state = stateTasks
		m.activeList = &m.tasksList
	}
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
	
	for _, t := range m.allTasks {
		if t.Parent != nil {
			continue // Hide subtasks from the main root list view
		}
		
		if query == "" {
			items = append(items, taskItem(t))
			continue
		}
		
		assignee := "unassigned"
		if len(t.Assignees) > 0 { assignee = strings.ToLower(t.Assignees[0].Username) }
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
			}
		}
	}
	
	totalPoints := 0.0
	for _, item := range items {
		if p := clickup.Task(item.(taskItem)).Points; p != nil {
			totalPoints += *p
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
		case "enter", "right":
			switch m.state {
			case stateTeams:
				if i, ok := m.activeList.SelectedItem().(teamItem); ok {
					m.selectedTeam = i.ID
					spaces, _ := m.client.GetSpaces(m.selectedTeam)
					m.allSpaces = spaces
					var items []list.Item
					for _, s := range spaces {
						items = append(items, spaceItem(s))
					}
					m.spacesList.SetItems(items)
					m.state = stateSpaces
					m.activeList = &m.spacesList
				}
			case stateSpaces:
				if i, ok := m.activeList.SelectedItem().(spaceItem); ok {
					m.selectedSpace = i.ID
					lists, _ := m.client.GetSpaceLists(m.selectedSpace)
					m.allLists = lists
					var items []list.Item
					for _, l := range lists {
						items = append(items, listItem(l))
					}
					m.listsList.SetItems(items)
					m.state = stateLists
					m.activeList = &m.listsList
				}
			case stateLists:
				if i, ok := m.activeList.SelectedItem().(listItem); ok {
					m.loadTasksAndSwitch(i.ID)
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
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			subtasks := m.getSubtasks(m.selectedTask.ID)
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(subtasks) {
				m.taskHistory = append(m.taskHistory, m.selectedTask)
				m.selectedTask = subtasks[idx]
				m.updateViewportContent()
			}
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

	help := lipgloss.NewStyle().Foreground(ColorSubtext).Render("q/esc/left: back | c: comment | 1-9: traverse subtasks")
	b.WriteString(help)

	m.vp.SetContent(b.String())
}

func (m *AppModel) View() string {
	if m.width == 0 {
		return "Starting..."
	}

	var mainContent string

	switch m.state {
	case stateTeams, stateSpaces, stateLists, stateTasks:
		mainContent = m.activeList.View()
	case stateTaskDetail:
		mainContent = m.vp.View()
	case stateComment:
		mainContent = m.vp.View() + "\n\n" + m.commentInput.View() + "\n(Enter to submit, Esc to cancel)"
	case stateCommand:
		if m.prevState == stateTaskDetail {
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

	fullUI := mainContent + "\n\n" + bottomBar
	return BaseStyle.Render(fullUI)
}
