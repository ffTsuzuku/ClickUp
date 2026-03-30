# Checklist TUI Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add dedicated checklist mode (`L` key) replacing slash commands with keyboard-driven TUI interaction

**Architecture:** Add `stateChecklist` to the existing Bubble Tea state machine. Flatten checklists to view items with headers. Context-aware key handling for item vs checklist operations. Inline editing via textinput.

**Tech Stack:** Go, Bubble Tea framework, Lip Gloss styling

---

## File Structure

- Modify: `ui/app.go` - Main state machine and model (3500+ lines)
  - Add `stateChecklist` constant
  - Add `checklistViewItem` struct and `checklistItemType` enum
  - Add model fields for checklist view state
  - Add message types for async operations
  - Add `flattenChecklists()` helper
  - Add `renderChecklistView()` method
  - Add `updateChecklist()` method
  - Wire up `L` key in task detail
- Modify: `ui/styles.go` - Add checklist-specific styles

---

## Task 1: Add State Constant and Types

**Files:**
- Modify: `ui/app.go:27-49`

- [ ] **Step 1: Add state constant**

Find the state constants block (around line 27-49):

```go
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
)
```

Add `stateChecklist` and `stateConfirmChecklistDelete` after `stateFilePicker`:

```go
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
```

- [ ] **Step 2: Add checklist view item types**

After the `filePickerItem` struct (around line 76-96), add:

```go
type checklistItemType int

const (
	checklistTypeHeader checklistItemType = iota
	checklistTypeItem
)

type checklistViewItem struct {
	itemType   checklistItemType
	checklist  clickup.Checklist
	item       clickup.ChecklistItem
	itemIndex  int
}

type checklistDeleteConfirmMsg struct {
	Checklist clickup.Checklist
}
```

- [ ] **Step 3: Add message types**

Find the message types section (around line 602-680) and add:

```go
type checklistItemUpdatedMsg struct{}
type checklistCreatedMsg struct{}
type checklistDeletedMsg struct{}
```

- [ ] **Step 4: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add state constant, types, and message types"
```

---

## Task 2: Add Model Fields

**Files:**
- Modify: `ui/app.go:531-600`

- [ ] **Step 1: Add checklist view fields to AppModel struct**

Find the `AppModel` struct (around line 531-600) and add these fields after `filePickerShowHidden`:

```go
filePickerShowHidden      bool
externalEditTarget        string

// Checklist view state
checklistViewItems       []checklistViewItem
checklistSelectedIdx     int
checklistEditingItem     *checklistViewItem
checklistEditInput       textinput.Model
checklistPendingDelete  string // checklist ID
checklistEditOriginal    string
```

- [ ] **Step 2: Initialize checklistEditInput in newBaseModel**

Find `newBaseModel()` function (around line 295-378) and add initialization:

```go
ci := textarea.New()
ci.Placeholder = "Enter comment..."
ci.SetWidth(80)
ci.SetHeight(5)

clEdit := textinput.New()
clEdit.Placeholder = "Item name..."
clEdit.CharLimit = 200
clEdit.Prompt = "  > "
```

Then in the struct initialization (around line 354-378), add:

```go
m := &AppModel{
	// ... existing fields ...
	checklistViewItems:   nil,
	checklistSelectedIdx: 0,
	checklistEditingItem: nil,
	checklistEditInput:   clEdit,
}
```

- [ ] **Step 3: Add to stateLabel() switch**

Find `stateLabel()` method (around line 221-264) and add:

```go
case stateFilePicker:
	return "Attachment File Picker"
case stateChecklist:
	return "Checklists"
case stateConfirmChecklistDelete:
	return "Confirm Delete Checklist"
```

- [ ] **Step 4: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add checklist model fields and initialization"
```

---

## Task 3: Add Helper Functions

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Add flattenChecklists helper**

Add after `getSubtasks()` method (around line 1733-1741):

```go
func (m *AppModel) flattenChecklists() {
	m.checklistViewItems = nil
	for _, cl := range m.selectedTask.Checklists {
		m.checklistViewItems = append(m.checklistViewItems, checklistViewItem{
			itemType: checklistTypeHeader,
			checklist: cl,
		})
		for i, item := range cl.Items {
			m.checklistViewItems = append(m.checklistViewItems, checklistViewItem{
				itemType:  checklistTypeItem,
				checklist: cl,
				item:      item,
				itemIndex: i,
			})
		}
	}
	if m.checklistSelectedIdx >= len(m.checklistViewItems) {
		m.checklistSelectedIdx = 0
	}
	if m.checklistSelectedIdx < 0 {
		m.checklistSelectedIdx = 0
	}
}
```

- [ ] **Step 2: Add checklistEditOriginal getter**

Add after `flattenChecklists()`:

```go
func (m *AppModel) isEditingChecklistItem() bool {
	return m.checklistEditingItem != nil
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
```

- [ ] **Step 3: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add flattenChecklists and edit helpers"
```

---

## Task 4: Add Checklist Styles

**Files:**
- Modify: `ui/styles.go`

- [ ] **Step 1: Add checklist-specific styles**

Add at end of file (after line 48):

```go
ChecklistHeaderStyle = lipgloss.NewStyle().
	Foreground(ColorPrimary).
	Bold(true)

ChecklistItemStyle = lipgloss.NewStyle().
	Foreground(ColorText)

ChecklistItemResolvedStyle = lipgloss.NewStyle().
	Foreground(ColorSubtext).
	Strikethrough(true)

ChecklistCheckboxStyle = lipgloss.NewStyle().
	Foreground(ColorPrimary)

ChecklistSelectedStyle = lipgloss.NewStyle().
	Background(ColorBorder).
	Foreground(ColorText)
```

- [ ] **Step 2: Commit**

```bash
git add ui/styles.go
git commit -m "feat(checklist): add checklist styles"
```

---

## Task 5: Add Checklist Rendering

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Add renderChecklistView method**

Add after `renderEditDesc()` method (around line 1779-1824):

```go
func (m *AppModel) renderChecklistView() string {
	var b strings.Builder
	
	if len(m.selectedTask.Checklists) == 0 {
		b.WriteString(TitleStyle.Render("Checklists"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("No checklists. Press 'n' to create one."))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("Press L or Esc to go back"))
		return b.String()
	}
	
	b.WriteString(TitleStyle.Render("Checklists"))
	b.WriteString("\n\n")
	
	indent := "  "
	checkboxUnchecked := lipgloss.NewStyle().Foreground(ColorPrimary).Render("[ ]")
	checkboxChecked := lipgloss.NewStyle().Foreground(ColorSecondary).Render("[x]")
	
	for idx, viewItem := range m.checklistViewItems {
		isSelected := idx == m.checklistSelectedIdx
		
		if viewItem.itemType == checklistTypeHeader {
			prefix := indent + "● "
			line := prefix + viewItem.checklist.Name
			if isSelected {
				line = ChecklistSelectedStyle.Render(">"+line[1:])
			} else {
				line = ChecklistHeaderStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		} else {
			checkbox := checkboxUnchecked
			itemStyle := ChecklistItemStyle
			if viewItem.item.Resolved {
				checkbox = checkboxChecked
				itemStyle = ChecklistItemResolvedStyle
			}
			line := fmt.Sprintf("%s%d. %s %s", indent, viewItem.itemIndex+1, checkbox, viewItem.item.Name)
			if isSelected {
				b.WriteString(ChecklistSelectedStyle.Render(line))
			} else {
				b.WriteString(itemStyle.Render(line))
			}
			b.WriteString("\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Italic(true).Render("↑↓ Navigate | Space Toggle | a Add | r Rename | d Delete | n New checklist | Esc Back"))
	
	return b.String()
}
```

- [ ] **Step 2: Add renderConfirmChecklistDelete method**

Add after `renderChecklistView()`:

```go
func (m *AppModel) renderConfirmChecklistDelete() string {
	var checklistName string
	for _, cl := range m.selectedTask.Checklists {
		if cl.ID == m.checklistPendingDelete {
			checklistName = cl.Name
			break
		}
	}
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Delete Checklist?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete '%s' and all its items?", checklistName))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render("[y] Delete  "))
	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext).Render("[n] Cancel"))
	return b.String()
}
```

- [ ] **Step 3: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add checklist view rendering"
```

---

## Task 6: Add Checklist Update Function

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Add updateChecklist method**

Add after `updateConfirmDiscardDesc()` method (around line 2757):

```go
func (m *AppModel) updateChecklist(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		
		if m.isEditingChecklistItem() {
			switch msg.Type {
			case tea.KeyEnter:
				newValue := strings.TrimSpace(m.checklistEditInput.Value())
				original := m.getChecklistEditOriginal()
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Blur()
				m.checklistEditingItem = nil
				
				if newValue != "" && newValue != original {
					m.loading = true
					m.loadingMsg = "Updating..."
					if m.checklistEditOriginal == m.checklistEditingItem.checklist.Name {
						checklist := m.checklistEditingItem.checklist
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.UpdateChecklist(checklist.ID, newValue); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					} else {
						item := m.checklistEditingItem.item
						checklist := m.checklistEditingItem.checklist
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							if err := m.client.UpdateChecklistItem(checklist.ID, item.ID, newValue, item.Resolved); err != nil {
								return errMsg(err)
							}
							return checklistItemUpdatedMsg{}
						})
					}
				}
				return m, nil
				
			case tea.KeyEsc:
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Blur()
				m.checklistEditingItem = nil
				return m, nil
			}
			
			var cmd tea.Cmd
			m.checklistEditInput, cmd = m.checklistEditInput.Update(msg)
			return m, cmd
		}
		
		switch s {
		case "esc", "q":
			m.checklistViewItems = nil
			m.checklistSelectedIdx = 0
			m.state = stateTaskDetail
			m.updateViewportContent()
			return m, nil
			
		case "up", "k":
			if m.checklistSelectedIdx > 0 {
				m.checklistSelectedIdx--
			}
			return m, nil
			
		case "down", "j":
			if m.checklistSelectedIdx < len(m.checklistViewItems)-1 {
				m.checklistSelectedIdx++
			}
			return m, nil
			
		case " ":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeItem {
					m.loading = true
					m.loadingMsg = "Toggling..."
					checklist := item.checklist
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.UpdateChecklistItem(checklist.ID, item.item.ID, item.item.Name, !item.item.Resolved); err != nil {
							return errMsg(err)
						}
						return checklistItemUpdatedMsg{}
					})
				}
			}
			return m, nil
			
		case "enter", "e":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditInput.SetValue(m.getChecklistEditOriginal())
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
				return m, textinput.Blink
			}
			return m, nil
			
		case "a":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				checklist := item.checklist
				if item.itemType == checklistTypeHeader {
					checklist = item.checklist
				} else {
					checklist = item.checklist
				}
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditInput.SetValue("")
				m.checklistEditInput.Placeholder = "New item name..."
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(0)
				return m, textinput.Blink
			}
			return m, nil
			
		case "r":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
				m.checklistEditInput.SetValue(m.getChecklistEditOriginal())
				m.checklistEditInput.Placeholder = "Name..."
				m.checklistEditInput.Focus()
				m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
				return m, textinput.Blink
			}
			return m, nil
			
		case "d":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeItem {
					m.loading = true
					m.loadingMsg = "Deleting item..."
					checklist := item.checklist
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.DeleteChecklistItem(checklist.ID, item.item.ID); err != nil {
							return errMsg(err)
						}
						return checklistItemUpdatedMsg{}
					})
				} else {
					m.checklistPendingDelete = item.checklist.ID
					m.state = stateConfirmChecklistDelete
					return m, nil
				}
			}
			return m, nil
			
		case "R", "D":
			if m.checklistSelectedIdx < len(m.checklistViewItems) {
				item := m.checklistViewItems[m.checklistSelectedIdx]
				if item.itemType == checklistTypeHeader {
					if s == "D" {
						m.checklistPendingDelete = item.checklist.ID
						m.state = stateConfirmChecklistDelete
						return m, nil
					}
					m.checklistEditingItem = &m.checklistViewItems[m.checklistSelectedIdx]
					m.checklistEditInput.SetValue(item.checklist.Name)
					m.checklistEditInput.Placeholder = "Checklist name..."
					m.checklistEditInput.Focus()
					m.checklistEditInput.SetCursor(len(m.checklistEditInput.Value()))
					return m, textinput.Blink
				}
			}
			return m, nil
			
		case "n":
			m.checklistEditInput.SetValue("")
			m.checklistEditInput.Placeholder = "New checklist name..."
			m.checklistEditInput.Focus()
			m.checklistEditInput.SetCursor(0)
			m.checklistEditingItem = nil
			m.checklistPendingDelete = ""
			return m, tea.Batch(textinput.Blink, func() tea.Msg {
				return checklistCreatedMsg{}
			})
		}
	}
	
	var cmd tea.Cmd
	m.checklistEditInput, cmd = m.checklistEditInput.Update(msg)
	return m, cmd
}
```

- [ ] **Step 2: Add updateConfirmChecklistDelete method**

Add after `updateChecklist()`:

```go
func (m *AppModel) updateConfirmChecklistDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			checklistID := m.checklistPendingDelete
			m.checklistPendingDelete = ""
			m.loading = true
			m.loadingMsg = "Deleting checklist..."
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				if err := m.client.DeleteChecklist(checklistID); err != nil {
					return errMsg(err)
				}
				return checklistDeletedMsg{}
			})
		case "n", "esc", "q":
			m.checklistPendingDelete = ""
			m.state = stateChecklist
			return m, nil
		}
	}
	return m, nil
}
```

- [ ] **Step 3: Handle checklistItemUpdatedMsg, checklistCreatedMsg, checklistDeletedMsg in Update**

Find the Update function's message handling (around line 1699-1730) and add cases after the existing message handlers:

```go
case checklistItemUpdatedMsg:
	m.loading = false
	return m, tea.Batch(
		fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState),
	)

case checklistCreatedMsg:
	m.loading = false
	if m.checklistEditingItem == nil && m.checklistEditInput.Value() != "" {
		name := strings.TrimSpace(m.checklistEditInput.Value())
		m.checklistEditInput.SetValue("")
		m.checklistEditInput.Blur()
		if name != "" {
			m.loading = true
			m.loadingMsg = "Creating checklist..."
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				if err := m.client.CreateChecklist(m.selectedTask.ID, name); err != nil {
					return errMsg(err)
				}
				return checklistItemUpdatedMsg{}
			})
		}
	} else if m.checklistEditingItem != nil && m.checklistEditInput.Value() != "" {
		newValue := strings.TrimSpace(m.checklistEditInput.Value())
		m.checklistEditInput.SetValue("")
		m.checklistEditInput.Blur()
		if newValue != "" {
			item := *m.checklistEditingItem
			m.checklistEditingItem = nil
			m.loading = true
			m.loadingMsg = "Adding item..."
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				if err := m.client.CreateChecklistItem(item.checklist.ID, newValue); err != nil {
					return errMsg(err)
				}
				return checklistItemUpdatedMsg{}
			})
		}
	}
	m.checklistEditingItem = nil
	return m, nil

case checklistDeletedMsg:
	m.loading = false
	return m, tea.Batch(
		fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.detailBackState),
	)
```

- [ ] **Step 4: Wire checklist states in main Update switch**

Find the Update function's state switch (around line 1699-1730) and add:

```go
case stateChecklist:
	return m.updateChecklist(msg)
case stateConfirmChecklistDelete:
	return m.updateConfirmChecklistDelete(msg)
```

- [ ] **Step 5: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add updateChecklist and updateConfirmChecklistDelete"
```

---

## Task 7: Wire Up Task Detail and View Integration

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Add `L` key binding in updateDetail**

Find `updateDetail()` method (around line 2359-2443) and add case in the switch:

```go
case "r":
	m.loading = true
	m.loadingMsg = "Refreshing task..."
	return m, tea.Batch(m.spinner.Tick, fetchTaskCmd(m.client, m.selectedTask.ID, m.selectedTeam, m.prevState))
case "L":
	if len(m.selectedTask.Checklists) > 0 {
		m.flattenChecklists()
		m.checklistSelectedIdx = 0
		m.checklistEditingItem = nil
		m.state = stateChecklist
		return m, nil
	}
	m.popupMsg = "No checklists on this task. Press 'n' from command mode to create one."
	return m, tea.Tick(time.Second*2, func(_ time.Time) tea.Msg { return clearPopupMsg{} })
```

- [ ] **Step 2: Wire checklist view into View()**

Find the `View()` method that renders content based on state. Look for where `stateTaskDetail` is rendered (around line 3530-3600). Add checklist state:

```go
case stateTaskDetail:
	return m.renderDetail()
case stateChecklist:
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderChecklistView()+"\n\n"+m.checklistEditInput.View())
case stateConfirmChecklistDelete:
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderConfirmChecklistDelete())
```

- [ ] **Step 3: Initialize checklistEditInput in newBaseModel**

Find `newBaseModel()` and add the initialization that was missed. Add to the model initialization:

```go
m := &AppModel{
	// ... existing fields ...
	checklistViewItems:   nil,
	checklistSelectedIdx: 0,
	checklistEditingItem: nil,
	checklistEditInput:   m.checklistEditInput, // Already initialized above
}
```

Wait - we already initialized `clEdit` earlier. Make sure it's assigned:

```go
clEdit := textinput.New()
clEdit.Placeholder = "Item name..."
clEdit.CharLimit = 200
clEdit.Prompt = "  > "

m := &AppModel{
	// ... existing fields ...
	checklistViewItems:   nil,
	checklistSelectedIdx: 0,
	checklistEditingItem: nil,
	checklistEditInput:   clEdit,
}
```

- [ ] **Step 4: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): wire up L key and checklist view rendering"
```

---

## Task 8: Update Help Content

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Add checklist help text**

Find `updateHelpContent()` method (around line 3721) and add checklist section:

```go
b.WriteString("• L     : Open checklist view\n")
b.WriteString("• ↑↓/jk : Navigate checklist items\n")
b.WriteString("• Space : Toggle item complete\n")
b.WriteString("• Enter : Edit item name\n")
b.WriteString("• a     : Add new item\n")
b.WriteString("• r     : Rename (item or checklist)\n")
b.WriteString("• d     : Delete item\n")
b.WriteString("• n     : Create new checklist\n")
b.WriteString("• R     : Rename checklist\n")
b.WriteString("• D     : Delete checklist\n")
b.WriteString("• Esc   : Exit to task detail\n\n")
```

- [ ] **Step 2: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): add checklist help text"
```

---

## Task 9: Remove Old Checklist Commands from Suggestions

**Files:**
- Modify: `ui/app.go`

- [ ] **Step 1: Remove checklist commands from suggestions**

Find `updateCommandSuggestions()` (around line 1903-1921) and remove these lines:

```go
sugs = append(sugs, Suggestion{"/checklist add ", "Create a checklist on this ticket"})
sugs = append(sugs, Suggestion{"/checklist rename ", "Rename a checklist by number"})
sugs = append(sugs, Suggestion{"/checklist delete ", "Delete a checklist by number"})
sugs = append(sugs, Suggestion{"/checklist item add ", "Add an item to a checklist"})
sugs = append(sugs, Suggestion{"/checklist item rename ", "Rename a checklist item"})
sugs = append(sugs, Suggestion{"/checklist item toggle ", "Toggle a checklist item"})
sugs = append(sugs, Suggestion{"/checklist item delete ", "Delete a checklist item"})
```

Replace with:

```go
sugs = append(sugs, Suggestion{"/checklist add ", "Create a checklist (or use 'n' in checklist view)"})
sugs = append(sugs, Suggestion{"L", "Open checklist view (when viewing task)"})
```

- [ ] **Step 2: Commit**

```bash
git add ui/app.go
git commit -m "feat(checklist): deprecate old slash commands in suggestions"
```

---

## Task 10: Build and Manual Test

- [ ] **Step 1: Build the application**

```bash
cd /Users/tsuzuku/git/anti_gravity/ClickUp
go build -o clickup-tui
```

Expected: No build errors

- [ ] **Step 2: Test scenarios manually**

1. Navigate to a task with checklists
2. Press `L` to enter checklist view
3. Navigate with ↑/↓ 
4. Press Space to toggle an item
5. Press Enter to edit an item name
6. Press `a` to add a new item
7. Press `r` to rename an item/checklist
8. Press `d` on an item to delete it
9. Press `D` on a checklist header to delete the whole checklist
10. Press `n` to create a new checklist
11. Press Esc to exit back to task detail

- [ ] **Step 3: Commit if all tests pass**

```bash
git add -A
git commit -m "feat(checklist): complete checklist TUI mode implementation"
```

---

## Verification Checklist

- [ ] `L` enters checklist mode from task detail
- [ ] All checklists and items render correctly
- [ ] ↑/↓ navigation works
- [ ] Space toggles item completion
- [ ] Enter starts inline edit for items
- [ ] `a` adds new item to current checklist
- [ ] `r` renames items and checklists
- [ ] `d` deletes items (no confirmation)
- [ ] `D` deletes checklists (with confirmation)
- [ ] `n` creates new checklist
- [ ] Esc exits back to task detail
- [ ] Completed items appear dimmed with strikethrough
- [ ] Help includes checklist keybindings
