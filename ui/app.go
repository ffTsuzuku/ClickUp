package ui

import (
	"fmt"
	"sort"
	"strings"

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

	clEdit := textinput.New()
	clEdit.Placeholder = "Item name..."
	clEdit.CharLimit = 200
	clEdit.Prompt = "  > "

	cmd := textinput.New()
	cmd.Placeholder = "Enter slash command (e.g. /filter) or /help..."
	cmd.Prompt = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("> ")
	cmd.CharLimit = 156
	cmd.Width = 50

	ti := textinput.New()
	ti.Placeholder = "Task name..."
	ti.CharLimit = 200

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
		state:              stateTeams,
		prevState:          stateTeams,
		cfg:                cfg,
		client:             clickup.NewClient(cfg.ClickupAPIKey),
		teamsList:          teamsList,
		spacesList:         spacesList,
		listsList:          listsList,
		tasksList:          tasksList,
		searchList:         searchList,
		fileList:           fileList,
		commentInput:       ci,
		checklistEditInput: clEdit,
		taskInput:          ti,
		descInput:          da,
		cmdInput:           cmd,
		vp:                 vp,
		renderer:           r,
		spinner:            s,
		currentUser:        "Unauthenticated",
		currentUserID:      0,
		activeProfile:      cfg.ActiveProfileName(),
	}
	m.activeList = &m.teamsList
	return m
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
			contentH -= (menuH + 1)
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
