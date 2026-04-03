package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) contentViewportSize() (int, int) {
	width := m.vp.Width
	height := m.vp.Height
	if width <= 0 {
		width = m.width
	}
	if height <= 0 {
		height = m.height
	}
	return width, height
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

func (m *AppModel) renderChecklistView() string {
	var b strings.Builder
	content, _, _ := m.buildChecklistView()
	b.WriteString(content)
	return b.String()
}

func (m *AppModel) buildChecklistView() (string, []int, []int) {
	var b strings.Builder
	var starts []int
	var ends []int
	currentLine := 0

	if len(m.selectedTask.Checklists) == 0 {
		b.WriteString(TitleStyle.Render("Checklists"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No checklists. Press 'n' to create one."))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("Press L or Esc to go back"))
		return b.String(), nil, nil
	}

	b.WriteString(TitleStyle.Render("Checklists"))
	b.WriteString("\n\n")
	currentLine += 3

	indent := "  "
	checkboxUnchecked := lipgloss.NewStyle().Foreground(ColorPrimary).Render("[ ]")
	checkboxChecked := lipgloss.NewStyle().Foreground(ColorSecondary).Render("[x]")

	for idx, viewItem := range m.checklistViewItems {
		isSelected := idx == m.checklistSelectedIdx

		if viewItem.itemType == checklistTypeHeader {
			prefix := indent + "● "
			line := prefix + viewItem.checklist.Name
			if isSelected {
				line = ChecklistSelectedStyle.Render(">" + line[1:])
			} else {
				line = ChecklistHeaderStyle.Render(line)
			}
			starts = append(starts, currentLine)
			b.WriteString(line)
			b.WriteString("\n")
			currentLine++
			ends = append(ends, currentLine-1)
		} else {
			checkbox := checkboxUnchecked
			itemStyle := ChecklistItemStyle
			if viewItem.item.Resolved {
				checkbox = checkboxChecked
				itemStyle = ChecklistItemResolvedStyle
			}

			padding := strings.Repeat("  ", viewItem.depth)
			prefix := fmt.Sprintf("%s%s%d. ", indent, padding, viewItem.itemIndex+1)
			name := viewItem.item.Name

			if isSelected {

				textStyle := ChecklistSelectedStyle
				if viewItem.item.Resolved {
					textStyle = textStyle.Copy().Foreground(ColorSubtext)
				}

				cbStyle := lipgloss.NewStyle().Background(ColorBorder).Foreground(ColorSecondary)
				cbText := "[x]"
				if !viewItem.item.Resolved {
					cbStyle = lipgloss.NewStyle().Background(ColorBorder).Foreground(ColorPrimary)
					cbText = "[ ]"
				}
				cbStr := cbStyle.Render(cbText)

				starts = append(starts, currentLine)
				b.WriteString(fmt.Sprintf("%s%s %s", textStyle.Render(prefix), cbStr, textStyle.Render(name)))
			} else {
				starts = append(starts, currentLine)
				b.WriteString(fmt.Sprintf("%s%s %s", itemStyle.Render(prefix), checkbox, itemStyle.Render(name)))
			}
			b.WriteString("\n")
			currentLine++
			ends = append(ends, currentLine-1)
		}
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("↑↓ Navigate | Space Toggle | Tab/Shift+Tab Indent | a Add | r Rename | d Delete | n New | q Back"))
	currentLine += 2

	return b.String(), starts, ends
}

func (m *AppModel) renderConfirmChecklistDelete() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Delete Checklist?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete '%s' and all its items?", m.checklistPendingDelete.Name))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render("[y] Delete  "))
	return b.String()
}

func (m *AppModel) renderCommentsView() string {
	var b strings.Builder
	content, _, _ := m.buildCommentsView()
	b.WriteString(content)
	return b.String()
}

func (m *AppModel) buildCommentsView() (string, []int, []int) {
	var b strings.Builder
	var starts []int
	var ends []int
	currentLine := 0

	if len(m.selectedComments) == 0 {
		b.WriteString(TitleStyle.Render("Comments"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No comments. Press 'c' to create one."))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("Press C or Esc to go back"))
		return b.String(), nil, nil
	}

	b.WriteString(TitleStyle.Render(fmt.Sprintf("Comments (%d)", len(m.selectedComments))))
	b.WriteString("\n\n")
	currentLine += 3

	commentWidth := m.width - 4
	if commentWidth < 24 {
		commentWidth = 24
	}

	for i, c := range m.selectedComments {
		isSelected := i == m.commentSelectedIdx

		authorName := c.User.Username
		if authorName == "" {
			authorName = "Unknown"
		}

		headerColor := ColorSecondary
		if isSelected {
			headerColor = ColorPrimary
		}

		header := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Foreground(headerColor).Bold(true).Render(fmt.Sprintf("ID: %d", i+1)),
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

		borderColor := ColorBorder
		if isSelected {
			borderColor = ColorPrimary
		}

		commentStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			Width(commentWidth)

		if c.Parent != nil && *c.Parent != "" {
			commentStyle = commentStyle.MarginLeft(2)
		}

		renderedComment := commentStyle.Render(header + "\n" + cardDivider + "\n" + message)
		starts = append(starts, currentLine)
		b.WriteString(renderedComment)
		b.WriteString("\n\n")
		currentLine += lipgloss.Height(renderedComment) + 2
		ends = append(ends, currentLine-1)
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("↑↓ Navigate | c Add | r Reply | e Edit | d Delete | q Back"))
	currentLine += 2

	return b.String(), starts, ends
}

func (m *AppModel) renderConfirmCommentDelete() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Delete Comment?"))
	b.WriteString("\n\n")
	if m.commentSelectedIdx >= 0 && m.commentSelectedIdx < len(m.selectedComments) {
		b.WriteString("Are you sure you want to delete comment ID: " + fmt.Sprintf("%d", m.commentSelectedIdx+1) + "?\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render(m.selectedComments[m.commentSelectedIdx].CommentText))
		b.WriteString("\n\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render("[y] Delete  "))
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("[n] Cancel"))
	return b.String()
}

func (m *AppModel) updateCommandSuggestions() {
	var sugs []Suggestion

	sugs = append(sugs, Suggestion{"/clear", "Clear active list filters"})
	sugs = append(sugs, Suggestion{"/help", "Show help documentation"})

	if m.prevState == stateTeams {
		sugs = append(sugs, Suggestion{"/filter", "Filter workspaces by name"})
	} else if m.prevState == stateSpaces {
		sugs = append(sugs, Suggestion{"/filter", "Filter spaces by name"})
		sugs = append(sugs, Suggestion{"/space", "Manage Spaces"})
		sugs = append(sugs, Suggestion{"/space create ", "Create a new Space"})
		sugs = append(sugs, Suggestion{"/space rename ", "Rename the highlighted Space"})
		sugs = append(sugs, Suggestion{"/space delete", "Delete the highlighted Space"})
	} else if m.prevState == stateLists {
		sugs = append(sugs, Suggestion{"/filter", "Filter lists by name"})
		sugs = append(sugs, Suggestion{"/list", "Manage Lists"})
		sugs = append(sugs, Suggestion{"/list create ", "Create a new List"})
		sugs = append(sugs, Suggestion{"/list rename ", "Rename the highlighted List"})
		sugs = append(sugs, Suggestion{"/list delete", "Delete the highlighted List"})
	} else if m.prevState == stateTasks {
		sugs = append(sugs, Suggestion{"/filter", "Filter tasks by text"})
		sugs = append(sugs, Suggestion{"/list", "Manage Lists"})
		sugs = append(sugs, Suggestion{"/list create ", "Create a new List"})
	}


	if m.prevState != stateTeams {
		sugs = append(sugs, Suggestion{"/ticket ", "Open a ticket directly by ID"})
		sugs = append(sugs, Suggestion{"/search ", "Search tickets across the workspace"})
	}

	sugs = append(sugs, Suggestion{"/profile", "Manage ClickUp profiles"})
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

	if m.prevState != stateTeams {
		sugs = append(sugs, Suggestion{"/search status:in progress", "Search tickets filtered by status"})
		sugs = append(sugs, Suggestion{"/search assignee:deep api", "Search tickets filtered by assignee plus text"})
	}

	if m.prevState == stateTeams || m.prevState == stateSpaces || m.prevState == stateLists {
		sugs = append(sugs, Suggestion{"/default set", "Set the currently highlighted item as your default routing"})
	}
	sugs = append(sugs, Suggestion{"/default", "Manage default routing"})
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
		sugs = append(sugs, Suggestion{"/edit title", "Edit the ticket title"})
		sugs = append(sugs, Suggestion{"/edit desc", "Edit the ticket description (inline)"})
		sugs = append(sugs, Suggestion{"/edit desc externally", "Edit description in $EDITOR (vim etc)"})
		sugs = append(sugs, Suggestion{"/copy title", "Copy the ticket title to your clipboard"})
		sugs = append(sugs, Suggestion{"/copy desc", "Copy the ticket description to your clipboard"})
		sugs = append(sugs, Suggestion{"/copy checklist", "Copy the ticket checklists to your clipboard"})
		sugs = append(sugs, Suggestion{"/copy all", "Copy the ticket context for AI prompting to your clipboard"})
		sugs = append(sugs, Suggestion{"/subtask", "Add a subtask to this ticket"})
		sugs = append(sugs, Suggestion{"/checklist", "Manage checklists"})
		sugs = append(sugs, Suggestion{"/checklist add ", "Create a checklist (or use 'n' in checklist view)"})
		sugs = append(sugs, Suggestion{"L", "Open checklist view (when viewing task)"})
		sugs = append(sugs, Suggestion{"C", "Open comments view (when viewing task)"})
		sugs = append(sugs, Suggestion{"/attach", "Manage attachments"})
		sugs = append(sugs, Suggestion{"/attach open ", "Open an attachment preview in your browser by number (e.g. /attach open 1)"})
		sugs = append(sugs, Suggestion{"/attach download ", "Download an attachment by number (e.g. /attach download 1)"})
		sugs = append(sugs, Suggestion{"/attach share ", "Copy an attachment URL to your clipboard by number (e.g. /attach share 1)"})
		sugs = append(sugs, Suggestion{"/attach upload", "Open a file browser to upload an attachment"})
		sugs = append(sugs, Suggestion{"/comment", "Manage comments"})
		sugs = append(sugs, Suggestion{"/comment edit ", "Edit a comment by its number (e.g. /comment edit 1)"})
		sugs = append(sugs, Suggestion{"/priority ", "Set task priority (urgent, high, normal, low, none)"})
		sugs = append(sugs, Suggestion{"/priority urgent", "Set priority to Urgent"})
		sugs = append(sugs, Suggestion{"/priority high", "Set priority to High"})
		sugs = append(sugs, Suggestion{"/priority normal", "Set priority to Normal"})
		sugs = append(sugs, Suggestion{"/priority low", "Set priority to Low"})
		sugs = append(sugs, Suggestion{"/priority none", "Clear task priority"})
		sugs = append(sugs, Suggestion{"/comment delete ", "Delete a comment by its number (e.g. /comment delete 1)"})
		sugs = append(sugs, Suggestion{"/comment reply ", "Reply to a comment by its number (e.g. /comment reply 1)"})

		for _, member := range m.teamMembers {
			sugs = append(sugs, Suggestion{"/assign " + strings.ToLower(member.User.Username), "Assign to " + member.User.Username})
		}
		if len(m.teamMembers) == 0 {

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
		for _, t := range m.allTeams {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find workspace " + t.Name})
		}
	} else if m.prevState == stateSpaces {
		for _, t := range m.allSpaces {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find space " + t.Name})
		}
	} else if m.prevState == stateLists {
		for _, t := range m.allLists {
			sugs = append(sugs, Suggestion{"/filter " + strings.ToLower(t.Name), "Find list " + t.Name})
		}
	}

	m.suggestions = sugs
	m.filterSuggestions()
}

func (m *AppModel) updateViewportContent() {
	var b strings.Builder

	id := m.selectedTask.ID
	if m.selectedTask.CustomID != "" {
		id = m.selectedTask.CustomID
	}

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
		commentWidth := m.width - 10
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
				lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(fmt.Sprintf("#%d", i+1)),
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
			b.WriteString("\n\n")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("Press 'C' to open Comments View for full management."))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No comments."))
	}
	b.WriteString("\n\n")

	m.vp.SetContent(b.String())
}

func (m *AppModel) updateCommentsViewportContent() {
	content, starts, ends := m.buildCommentsView()
	m.vp.SetContent(content)

	if len(starts) == 0 || m.commentSelectedIdx < 0 || m.commentSelectedIdx >= len(starts) {
		return
	}

	selectedStart := starts[m.commentSelectedIdx]
	selectedEnd := ends[m.commentSelectedIdx]
	visibleStart := m.vp.YOffset
	visibleEnd := visibleStart + max(0, m.vp.Height-1)

	if selectedStart < visibleStart {
		m.vp.SetYOffset(selectedStart)
		return
	}
	if selectedEnd > visibleEnd {
		m.vp.SetYOffset(selectedEnd - max(0, m.vp.Height-1))
	}
}

func (m *AppModel) updateChecklistViewportContent() {
	content, starts, ends := m.buildChecklistView()
	m.vp.SetContent(content)

	if len(starts) == 0 || m.checklistSelectedIdx < 0 || m.checklistSelectedIdx >= len(starts) {
		return
	}

	selectedStart := starts[m.checklistSelectedIdx]
	selectedEnd := ends[m.checklistSelectedIdx]
	visibleStart := m.vp.YOffset
	visibleEnd := visibleStart + max(0, m.vp.Height-1)

	if selectedStart < visibleStart {
		m.vp.SetYOffset(selectedStart)
		return
	}
	if selectedEnd > visibleEnd {
		m.vp.SetYOffset(selectedEnd - max(0, m.vp.Height-1))
	}
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
	b.WriteString("• /space delete            : Delete the highlighted Space after confirmation\n")
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
	b.WriteString("• /edit title      : Edit the ticket title\n")
	b.WriteString("• /edit desc       : Edit description (inline)\n")
	b.WriteString("• /edit desc externally : Edit description in external $EDITOR\n")
	b.WriteString("• /copy title      : Copy ticket title to clipboard\n")
	b.WriteString("• /copy desc       : Copy ticket description to clipboard\n")
	b.WriteString("• /copy checklist  : Copy ticket checklists to clipboard\n")
	b.WriteString("• /copy all        : Copy ticket context for AI prompting\n")
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
		b.WriteString("• A                : Copy ticket context for AI prompting\n")
		b.WriteString("• a                : Create a new subtask\n")
		b.WriteString("• s                : Copy ticket URL to clipboard\n")
	b.WriteString("• r                : Refresh current view from API\n")
	b.WriteString("• L                : Open checklist view\n")
	b.WriteString("• C                : Open comments view\n")
	b.WriteString("• ↑↓/jk            : Navigate checklist items\n")
	b.WriteString("• Space            : Toggle item complete\n")
	b.WriteString("• Enter            : Edit item name\n")
	b.WriteString("• a                : Add new item\n")
	b.WriteString("• r                : Rename (item or checklist)\n")
	b.WriteString("• d                : Delete item\n")
	b.WriteString("• n                : Create new checklist\n")
	b.WriteString("• R                : Rename checklist\n")
	b.WriteString("• D                : Delete checklist\n")
	b.WriteString("• Esc              : Exit to task detail\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("List/Space Actions"))
	b.WriteString("\n")
	b.WriteString("• a                : Create a new Space (in Spaces view)\n")
	b.WriteString("• e                : Rename highlighted Space (in Spaces view)\n")
	b.WriteString("• d                : Delete highlighted Space (in Spaces view)\n")
	b.WriteString("• a / n            : Create a new task (in Tasks view)\n")
	b.WriteString("• r                : Refresh the current view\n\n")

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
	case stateCommentsView:
		parts = append(parts, "Comments")
	case stateConfirmCommentDelete:
		parts = append(parts, "Comments", "Delete Comment")
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
	case stateConfirmSpaceDelete:
		parts = append(parts, "Spaces", "Delete Space")
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
		} else if m.state == stateSpaces {
			lines := strings.Split(view, "\n")
			lastIdx := len(lines) - 1
			for lastIdx >= 0 && strings.TrimSpace(lines[lastIdx]) == "" {
				lastIdx--
			}
			if lastIdx >= 0 {
				style := lipgloss.NewStyle().Foreground(ColorSubtext)
				lines[lastIdx] = strings.TrimRight(lines[lastIdx], " ") + style.Render(" • a/n: new • e: rename • d: delete • r: refresh")
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
		hint := lipgloss.NewStyle().Foreground(ColorSubtext).Render("q: back • a: subtask • c: comment • o: open • s: share • r: refresh")
		mainContent = m.vp.View() + "\n" + hint
	case stateChecklist:
		mainContent = m.vp.View()
	case stateConfirmChecklistDelete:
		contentWidth, contentHeight := m.contentViewportSize()
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, m.renderConfirmChecklistDelete())
	case stateCommentsView:
		mainContent = m.vp.View()
	case stateConfirmCommentDelete:
		contentWidth, contentHeight := m.contentViewportSize()
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, m.renderConfirmCommentDelete())
	case stateHelp:
		mainContent = m.vp.View()
	case stateComment:
		header := TitleStyle.Render("Adding Comment:")
		if m.replyToUser != "" {
			header = TitleStyle.Render(fmt.Sprintf("Replying to %s:", m.replyToUser))
		}
		bg := m.vp.View()
		if m.commentReturnState == stateCommentsView {
			bg = m.vp.View()
		}
		mainContent = bg + "\n\n" + header + "\n" + m.commentInput.View() + "\n(Ctrl+S to submit, Ctrl+E for Vim, Esc to cancel)"
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
		bg := m.vp.View()
		if m.commentReturnState == stateCommentsView {
			bg = m.vp.View()
		}
		mainContent = bg + "\n\n" + TitleStyle.Render("Editing Comment:") + "\n" + m.commentInput.View() + "\n(Ctrl+S to save, Ctrl+E for Vim, Esc to cancel)"
	case stateConfirmProfileDelete:
		contentWidth, contentHeight := m.contentViewportSize()
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Delete Profile?") + "\n\n" +
					fmt.Sprintf("Delete profile %q?", m.pendingDeleteProfile) + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: yes • n/esc: no"),
			)
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, box)
	case stateConfirmListDelete:
		contentWidth, contentHeight := m.contentViewportSize()
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Delete List?") + "\n\n" +
					fmt.Sprintf("Delete list %q?", m.pendingDeleteListName) + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: yes • n/esc: no"),
			)
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, box)
	case stateConfirmSpaceDelete:
		contentWidth, contentHeight := m.contentViewportSize()
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Delete Space?") + "\n\n" +
					fmt.Sprintf("Delete space %q?", m.pendingDeleteSpaceName) + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: yes • n/esc: no"),
			)
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, box)
	case stateConfirmDiscardDesc:
		contentWidth, contentHeight := m.contentViewportSize()
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Render(
				TitleStyle.Render("Discard Description Changes?") + "\n\n" +
					"Your unsaved description edits will be lost." + "\n\n" +
					lipgloss.NewStyle().Foreground(ColorSubtext).Render("y/enter: discard • n/esc: keep editing"),
			)
		mainContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, box)
	case stateCommand:
		if m.prevState == stateTaskDetail || m.prevState == stateHelp {
			mainContent = m.vp.View()
		} else if m.prevState == stateChecklist || m.prevState == stateConfirmChecklistDelete {
			mainContent = m.vp.View()
		} else if m.prevState == stateCommentsView || m.prevState == stateConfirmCommentDelete {
			mainContent = m.vp.View()
		} else {
			mainContent = m.activeList.View()
		}
	}

	var sb strings.Builder
	if m.state == stateCommand && len(m.filteredSuggest) > 0 {
		sb.WriteString("\n")
		
		startIdx := 0
		endIdx := len(m.filteredSuggest)
		hasMoreItems := len(m.filteredSuggest) > 10
		
		if hasMoreItems {
			startIdx = m.suggestIdx - 4
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + 10
			if endIdx > len(m.filteredSuggest) {
				endIdx = len(m.filteredSuggest)
				startIdx = endIdx - 10
			}
		}

		if hasMoreItems {
			if startIdx > 0 {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("▲") + "\n")
			} else {
				sb.WriteString("\n")
			}
		}

		for i := startIdx; i < endIdx; i++ {
			s := m.filteredSuggest[i]
			textStyle := lipgloss.NewStyle().Width(50).Foreground(ColorPrimary).Bold(i == m.suggestIdx)
			descStyle := lipgloss.NewStyle().Foreground(ColorText).PaddingLeft(2)

			if i == m.suggestIdx {
				textStyle = textStyle.Foreground(ColorSecondary)
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorSecondary).Render("> "))
			} else {
				sb.WriteString("  ")
			}

			sb.WriteString(textStyle.Render(s.Text))
			sb.WriteString(descStyle.Render(s.Desc))

			if i < endIdx-1 || hasMoreItems {
				sb.WriteString("\n")
			}
		}

		if hasMoreItems {
			if endIdx < len(m.filteredSuggest) {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("▼"))
			} else {
				sb.WriteString(" ")
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
