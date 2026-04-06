package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

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

func splitQuotedFields(raw string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote rune
	inField := false
	escaped := false

	for _, r := range raw {
		switch {
		case escaped:
			current.WriteRune(r)
			inField = true
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			inField = true
		case r == '"' || r == '\'':
			quote = r
			inField = true
		case unicode.IsSpace(r):
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		default:
			current.WriteRune(r)
			inField = true
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields, nil
}

func parseProfileCreateInput(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", nil
	}

	fields, err := splitQuotedFields(raw)
	if err != nil {
		return "", "", err
	}
	if len(fields) == 0 {
		return "", "", nil
	}
	if len(fields) == 1 {
		return fields[0], "", nil
	}

	hasQuotes := strings.ContainsAny(raw, `"'`)
	if hasQuotes {
		return fields[0], strings.Join(fields[1:], " "), nil
	}
	return fields[0], strings.Join(fields[1:], " "), nil
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
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].DateCreated != "" && res[j].DateCreated != "" && res[i].DateCreated != res[j].DateCreated {
			return res[i].DateCreated < res[j].DateCreated
		}
		return res[i].ID < res[j].ID
	})
	return res
}

func buildChecklistViewItems(checklists []clickup.Checklist) []checklistViewItem {
	var viewItems []checklistViewItem
	for _, cl := range checklists {
		viewItems = append(viewItems, checklistViewItem{
			itemType:  checklistTypeHeader,
			checklist: cl,
		})

		itemByID := make(map[string]clickup.ChecklistItem, len(cl.Items))
		sourceOrder := make(map[string]int, len(cl.Items))
		nextSourceOrder := 0

		for _, item := range cl.Items {
			itemByID[item.ID] = item
			sourceOrder[item.ID] = nextSourceOrder
			nextSourceOrder++
		}

		var collectChildren func(parentID string, children []clickup.ChecklistItem)
		collectChildren = func(parentID string, children []clickup.ChecklistItem) {
			for _, child := range children {
				if _, ok := itemByID[child.ID]; !ok {
					if child.Parent == nil || *child.Parent == "" {
						inferredParentID := parentID
						child.Parent = &inferredParentID
					}
					itemByID[child.ID] = child
					sourceOrder[child.ID] = nextSourceOrder
					nextSourceOrder++
				}
				if len(child.Children) > 0 {
					collectChildren(child.ID, child.Children)
				}
			}
		}
		for _, item := range cl.Items {
			if len(item.Children) > 0 {
				collectChildren(item.ID, item.Children)
			}
		}

		var topLevel []clickup.ChecklistItem
		childrenMap := make(map[string][]clickup.ChecklistItem, len(itemByID))

		for _, item := range itemByID {
			if item.Parent != nil && *item.Parent != "" {
				childrenMap[*item.Parent] = append(childrenMap[*item.Parent], item)
			} else {
				topLevel = append(topLevel, item)
			}
		}

		sortChecklistItems := func(items []clickup.ChecklistItem) {
			sort.SliceStable(items, func(i, j int) bool {
				if items[i].DateCreated != "" && items[j].DateCreated != "" && items[i].DateCreated != items[j].DateCreated {
					return items[i].DateCreated < items[j].DateCreated
				}
				if items[i].OrderIndex != nil && items[j].OrderIndex != nil && *items[i].OrderIndex != *items[j].OrderIndex {
					return *items[i].OrderIndex < *items[j].OrderIndex
				}
				return sourceOrder[items[i].ID] < sourceOrder[items[j].ID]
			})
		}
		sortChecklistItems(topLevel)
		for parentID := range childrenMap {
			sortChecklistItems(childrenMap[parentID])
		}

		var traverse func(items []clickup.ChecklistItem, depth int)
		itemIndex := 0
		traverse = func(items []clickup.ChecklistItem, depth int) {
			for _, item := range items {
				viewItems = append(viewItems, checklistViewItem{
					itemType:  checklistTypeItem,
					checklist: cl,
					item:      item,
					itemIndex: itemIndex,
					depth:     depth,
				})
				itemIndex++

				if children, ok := childrenMap[item.ID]; ok {
					traverse(children, depth+1)
				}
			}
		}
		traverse(topLevel, 0)
	}
	return viewItems
}

func (m *AppModel) flattenChecklists() {
	m.checklistViewItems = buildChecklistViewItems(m.selectedTask.Checklists)
	if m.checklistSelectedIdx >= len(m.checklistViewItems) {
		m.checklistSelectedIdx = 0
	}
	if m.checklistSelectedIdx < 0 {
		m.checklistSelectedIdx = 0
	}
}

func (m *AppModel) applyChecklistSelectionTarget() {
	if m.checklistSelection == nil {
		return
	}

	target := m.checklistSelection
	m.checklistSelection = nil

	if target.selectLastItem && target.checklistID != "" {
		for idx := len(m.checklistViewItems) - 1; idx >= 0; idx-- {
			item := m.checklistViewItems[idx]
			if item.itemType == checklistTypeItem && item.checklist.ID == target.checklistID {
				m.checklistSelectedIdx = idx
				return
			}
		}
	}

	if target.selectLastChecklist {
		for idx := len(m.checklistViewItems) - 1; idx >= 0; idx-- {
			if m.checklistViewItems[idx].itemType == checklistTypeHeader {
				m.checklistSelectedIdx = idx
				return
			}
		}
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

func memberUsername(member clickup.Member) string {
	return strings.TrimSpace(member.User.Username)
}

func memberUserID(member clickup.Member) int {
	return member.User.ID
}

func memberFromAssignee(user clickup.Assignee) clickup.Member {
	var member clickup.Member
	member.User.ID = user.ID
	member.User.Username = user.Username
	return member
}

func (m *AppModel) mentionableMembers() []clickup.Member {
	var members []clickup.Member
	seen := make(map[int]bool)

	addMember := func(member clickup.Member) {
		id := memberUserID(member)
		name := memberUsername(member)
		if id == 0 || name == "" || seen[id] {
			return
		}
		seen[id] = true
		members = append(members, member)
	}

	for _, member := range m.teamMembers {
		addMember(member)
	}

	addMember(memberFromAssignee(m.selectedTask.Creator))
	for _, assignee := range m.selectedTask.Assignees {
		addMember(memberFromAssignee(assignee))
	}

	for _, comment := range m.selectedComments {
		addMember(memberFromAssignee(comment.User))
	}

	for _, task := range m.allTasks {
		addMember(memberFromAssignee(task.Creator))
		for _, assignee := range task.Assignees {
			addMember(memberFromAssignee(assignee))
		}
	}

	if m.currentUserID != 0 && m.currentUser != "" {
		var member clickup.Member
		member.User.ID = m.currentUserID
		member.User.Username = m.currentUser
		addMember(member)
	}

	return members
}

func (m *AppModel) cursorTextIndex() int {
	lines := strings.Split(m.commentInput.Value(), "\n")
	line := m.commentInput.Line()
	if line < 0 {
		return 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}

	index := 0
	for i := 0; i < line; i++ {
		index += len([]rune(lines[i])) + 1
	}
	return index + m.commentInput.LineInfo().CharOffset
}

func isMentionBoundary(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(".,!?:;()[]{}<>\"'`/\\|", r)
}

func isMentionQueryRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}

func (m *AppModel) activeMentionQuery() (int, int, string, bool) {
	cursor := m.cursorTextIndex()
	runes := []rune(m.commentInput.Value())
	if cursor < 0 || cursor > len(runes) {
		return 0, 0, "", false
	}

	start := -1
	for i := cursor - 1; i >= 0; i-- {
		r := runes[i]
		if r == '@' {
			if i > 0 {
				prev := runes[i-1]
				if !isMentionBoundary(prev) {
					return 0, 0, "", false
				}
			}
			start = i
			break
		}
		if isMentionBoundary(r) {
			return 0, 0, "", false
		}
		if !isMentionQueryRune(r) {
			return 0, 0, "", false
		}
	}
	if start == -1 {
		return 0, 0, "", false
	}

	queryRunes := runes[start+1 : cursor]
	for _, r := range queryRunes {
		if !isMentionQueryRune(r) {
			return 0, 0, "", false
		}
	}

	return start, cursor, string(queryRunes), true
}

func scoreMentionMatch(query, username string) int {
	query = normalizeSearchValue(query)
	username = normalizeSearchValue(username)
	if username == "" {
		return -1
	}
	if query == "" {
		return 1
	}
	switch {
	case username == query:
		return 100
	case strings.HasPrefix(username, query):
		return 80
	case strings.Contains(username, query):
		return 60
	case fuzzyMatch(query, username):
		return 40
	default:
		return -1
	}
}

func (m *AppModel) refreshMentionSuggestions() {
	m.mentionSuggestions = nil
	m.mentionSelectedIdx = 0
	m.mentionQuery = ""
	m.mentionQueryStart = 0
	m.mentionQueryEnd = 0

	start, end, query, ok := m.activeMentionQuery()
	members := m.mentionableMembers()
	if !ok || len(members) == 0 {
		return
	}

	type rankedMember struct {
		member clickup.Member
		score  int
	}
	var ranked []rankedMember
	seen := make(map[int]bool, len(members))
	for _, member := range members {
		id := memberUserID(member)
		name := memberUsername(member)
		if id == 0 || name == "" || seen[id] {
			continue
		}
		seen[id] = true
		score := scoreMentionMatch(query, name)
		if score < 0 {
			continue
		}
		ranked = append(ranked, rankedMember{member: member, score: score})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return strings.ToLower(memberUsername(ranked[i].member)) < strings.ToLower(memberUsername(ranked[j].member))
	})

	limit := min(6, len(ranked))
	for i := 0; i < limit; i++ {
		m.mentionSuggestions = append(m.mentionSuggestions, ranked[i].member)
	}
	if len(m.mentionSuggestions) == 0 {
		return
	}

	m.mentionQuery = query
	m.mentionQueryStart = start
	m.mentionQueryEnd = end
}

func commentPartsFromText(comment string, members []clickup.Member) []clickup.CommentBodyPart {
	runes := []rune(comment)
	if len(runes) == 0 {
		return nil
	}

	type mentionCandidate struct {
		member  clickup.Member
		name    string
		runeLen int
	}
	var candidates []mentionCandidate
	seen := make(map[int]bool, len(members))
	for _, member := range members {
		name := memberUsername(member)
		id := memberUserID(member)
		if name == "" || id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		candidates = append(candidates, mentionCandidate{
			member:  member,
			name:    name,
			runeLen: len([]rune(name)),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].runeLen != candidates[j].runeLen {
			return candidates[i].runeLen > candidates[j].runeLen
		}
		return strings.ToLower(candidates[i].name) < strings.ToLower(candidates[j].name)
	})

	var parts []clickup.CommentBodyPart
	var plain strings.Builder

	flushPlain := func() {
		if plain.Len() == 0 {
			return
		}
		parts = append(parts, clickup.CommentBodyPart{Text: plain.String()})
		plain.Reset()
	}

	for i := 0; i < len(runes); {
		if runes[i] != '@' {
			plain.WriteRune(runes[i])
			i++
			continue
		}
		if i > 0 && !isMentionBoundary(runes[i-1]) {
			plain.WriteRune(runes[i])
			i++
			continue
		}

		var matched *mentionCandidate
		matchEnd := i + 1
		for idx := range candidates {
			end := i + 1 + candidates[idx].runeLen
			if end > len(runes) {
				continue
			}
			if !strings.EqualFold(string(runes[i+1:end]), candidates[idx].name) {
				continue
			}
			if end < len(runes) && !isMentionBoundary(runes[end]) {
				continue
			}
			matched = &candidates[idx]
			matchEnd = end
			break
		}

		if matched == nil {
			plain.WriteRune(runes[i])
			i++
			continue
		}

		flushPlain()
		parts = append(parts, clickup.CommentBodyPart{
			Type: "tag",
			User: &clickup.CommentTaggedUser{ID: memberUserID(matched.member)},
		})
		i = matchEnd
	}

	flushPlain()
	if len(parts) == 1 && parts[0].Type == "" {
		return nil
	}
	return parts
}
