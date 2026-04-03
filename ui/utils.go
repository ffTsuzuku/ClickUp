package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

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

func (m *AppModel) getSubtasks(parentID string) []clickup.Task {
	var res []clickup.Task
	for _, t := range m.allTasks {
		if t.Parent != nil && *t.Parent == parentID {
			res = append(res, t)
		}
	}
	return res
}

func (m *AppModel) flattenChecklists() {
	m.checklistViewItems = nil
	for _, cl := range m.selectedTask.Checklists {
		m.checklistViewItems = append(m.checklistViewItems, checklistViewItem{
			itemType:  checklistTypeHeader,
			checklist: cl,
		})

		var topLevel []clickup.ChecklistItem
		childrenMap := make(map[string][]clickup.ChecklistItem)

		for _, item := range cl.Items {
			if item.Parent != nil && *item.Parent != "" {
				childrenMap[*item.Parent] = append(childrenMap[*item.Parent], item)
			} else {
				topLevel = append(topLevel, item)
			}
		}

		var traverse func(items []clickup.ChecklistItem, depth int)
		itemIndex := 0
		traverse = func(items []clickup.ChecklistItem, depth int) {
			for _, item := range items {
				m.checklistViewItems = append(m.checklistViewItems, checklistViewItem{
					itemType:  checklistTypeItem,
					checklist: cl,
					item:      item,
					itemIndex: itemIndex,
					depth:     depth,
				})
				itemIndex++

				if children, ok := childrenMap[item.ID]; ok {
					traverse(children, depth+1)
				} else if len(item.Children) > 0 {
					traverse(item.Children, depth+1)
				}
			}
		}
		traverse(topLevel, 0)
	}
	if m.checklistSelectedIdx >= len(m.checklistViewItems) {
		m.checklistSelectedIdx = 0
	}
	if m.checklistSelectedIdx < 0 {
		m.checklistSelectedIdx = 0
	}
}

func (m *AppModel) isEditingChecklistItem() bool {
	return m.checklistEditInput.Focused()
}

func (m *AppModel) getChecklistEditOriginal() string {
	if m.checklistEditingItem == nil {
		return ""
	}
	if m.checklistEditingItem.itemType == checklistTypeHeader {
		return m.checklistEditingItem.checklist.Name
	}
	return m.checklistEditingItem.item.Name
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

func (m *AppModel) filterSuggestions() {
	v := strings.ToLower(m.cmdInput.Value())
	words := strings.Split(v, " ")

	m.filteredSuggest = nil
	for _, s := range m.suggestions {
		text := strings.ToLower(s.Text)

		// Hide sub-commands (multi-word) unless base command is fully typed
		sParts := strings.Fields(text)
		if len(sParts) > 1 {
			baseCmd := sParts[0]
			if !(strings.HasPrefix(v, baseCmd+" ") || v == baseCmd) {
				continue
			}
		}

		match := true
		for _, w := range words {
			if w != "" && !strings.Contains(text, w) {
				match = false
				break
			}
		}
		if match {
			m.filteredSuggest = append(m.filteredSuggest, s)
		}
	}
	m.suggestIdx = 0
	m.updateLayout()
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

func flattenComments(comments []clickup.Comment) []clickup.Comment {
	var flat []clickup.Comment
	for _, c := range comments {
		flat = append(flat, c)
		if len(c.Replies) > 0 {

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
