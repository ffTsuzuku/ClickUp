package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsuzuku/clickup-tui/clickup"
)

// Item Wrappers
type teamItem clickup.Team

func (t teamItem) Title() string { return t.Name }

func (t teamItem) Description() string { return "Workspace (" + t.ID + ")" }

func (t teamItem) FilterValue() string { return t.Name }

type spaceItem clickup.Space

func (t spaceItem) Title() string { return t.Name }

func (t spaceItem) Description() string { return "Space" }

func (t spaceItem) FilterValue() string { return t.Name }

type listItem clickup.List

func (t listItem) Title() string { return t.Name }

func (t listItem) Description() string { return "List" }

func (t listItem) FilterValue() string { return t.Name }

type folderItem clickup.Folder

func (f folderItem) Title() string { return f.Name }

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

type checklistViewItem struct {
	itemType  checklistItemType
	checklist clickup.Checklist
	item      clickup.ChecklistItem
	itemIndex int
	depth     int
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
