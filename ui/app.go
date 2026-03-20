package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tsuzuku/clickup-tui/clickup"
)

type state int

const (
	stateList state = iota
	stateDetail
	stateComment
)

// Item wrapper for bubble list
type taskItem clickup.Task

func (t taskItem) Title() string       { return t.Name }
func (t taskItem) Description() string { return fmt.Sprintf("Status: %s | Assignee: %s", t.Status.Status, t.Assignee) }
func (t taskItem) FilterValue() string { return t.Name }

// AppModel is the main Bubble Tea model
type AppModel struct {
	state      state
	client     *clickup.Client
	taskList   list.Model
	input      textinput.Model
	vp         viewport.Model
	width      int
	height     int
	selected   clickup.Task
	err        error
}

func InitialModel() *AppModel {
	// Initialize API Client (Mock for now)
	c := clickup.NewClient("DUMMY_TOKEN")
	tasks, _ := c.GetTasks()

	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = taskItem(t)
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "ClickUp Tasks"
	l.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Placeholder = "Type a comment..."
	ti.CharLimit = 156
	ti.Width = 40

	vp := viewport.New(0, 0)
	vp.Style = DetailStyle

	return &AppModel{
		state:    stateList,
		client:   c,
		taskList: l,
		input:    ti,
		vp:       vp,
	}
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update sub-models size
		h, v := BaseStyle.GetFrameSize()
		m.taskList.SetSize(m.width-h, m.height-v)
		m.vp.Width = m.width - h
		m.vp.Height = m.height - v - 4 // Leave space for headers/footers
	}

	switch m.state {
	case stateList:
		return m.updateList(msg)
	case stateDetail:
		return m.updateDetail(msg)
	case stateComment:
		return m.updateComment(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.taskList.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if i, ok := m.taskList.SelectedItem().(taskItem); ok {
				m.selected = clickup.Task(i)
				m.state = stateDetail
				m.updateViewportContent()
				return m, nil
			}
		}
	}
	
	var cmd tea.Cmd
	m.taskList, cmd = m.taskList.Update(msg)
	return m, cmd
}

func (m *AppModel) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.state = stateList
			return m, nil
		case "c":
			m.state = stateComment
			m.input.Focus()
			return m, textinput.Blink
		case "e":
			// Basic toggle for status completion mock
			newStatus := "done"
			if m.selected.Status.Status == "done" {
				newStatus = "to do"
			}
			m.client.UpdateStatus(m.selected.ID, newStatus)
			
			// Refresh list
			tasks, _ := m.client.GetTasks()
			items := make([]list.Item, len(tasks))
			for i, t := range tasks {
				items[i] = taskItem(t)
				if t.ID == m.selected.ID {
					m.selected = t // Update local selection
				}
			}
			m.taskList.SetItems(items)
			m.updateViewportContent()
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
			v := m.input.Value()
			if v != "" {
				m.client.AddComment(m.selected.ID, v)
				m.input.SetValue("")
				m.input.Blur()
				m.state = stateDetail
				
				// Refresh selected task
				tasks, _ := m.client.GetTasks()
				for _, t := range tasks {
					if t.ID == m.selected.ID {
						m.selected = t
					}
				}
				m.updateViewportContent()
			}
			return m, nil
		case tea.KeyEsc:
			m.input.SetValue("")
			m.input.Blur()
			m.state = stateDetail
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *AppModel) updateViewportContent() {
	var b strings.Builder
	
	b.WriteString(TitleStyle.Render(fmt.Sprintf("[%s] %s", m.selected.Status.Status, m.selected.Name)))
	b.WriteString("\n\n")
	
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Assignee: "))
	b.WriteString(m.selected.Assignee)
	b.WriteString("\n\n")
	
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Description:\n"))
	b.WriteString(m.selected.Desc)
	b.WriteString("\n\n")
	
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Comments:\n"))
	if len(m.selected.Comments) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No comments yet."))
	} else {
		for _, c := range m.selected.Comments {
			b.WriteString(fmt.Sprintf("• %s (%s): %s\n", c.User, lipgloss.NewStyle().Foreground(ColorSubtext).Render("latest"), c.Comment))
		}
	}
	
	b.WriteString("\n\n")
	help := lipgloss.NewStyle().Foreground(ColorSubtext).Render("q/esc: back | c: comment | e: toggle status")
	b.WriteString(help)

	m.vp.SetContent(b.String())
}

func (m *AppModel) View() string {
	if m.width == 0 {
		return "Starting..."
	}

	var content string

	switch m.state {
	case stateList:
		content = m.taskList.View()
	case stateDetail:
		content = m.vp.View()
	case stateComment:
		content = m.vp.View() + "\n\n" + m.input.View() + "\n(Enter to submit, Esc to cancel)"
	}

	return BaseStyle.Render(content)
}
