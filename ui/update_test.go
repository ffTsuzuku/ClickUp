package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

func hasSuggestion(suggestions []Suggestion, text string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Text == text {
			return true
		}
	}
	return false
}

func newTestModel(t *testing.T) *AppModel {
	t.Helper()

	m := newBaseModel(&config.Config{})
	m.width = 120
	m.height = 40
	m.updateLayout()
	return m
}

func makeTask(id, name string) clickup.Task {
	return clickup.Task{
		ID:   id,
		Name: name,
		Status: clickup.TaskStatus{
			Status: "open",
		},
	}
}

func TestTaskDetailMsgResetsViewportWhenTaskChanges(t *testing.T) {
	m := newTestModel(t)

	oldTask := makeTask("old-task", "Old Task")
	oldTask.Desc = strings.Repeat("old line\n", 80)
	m.selectedTask = oldTask
	m.state = stateTaskDetail
	m.teamMembersTaskID = oldTask.ID
	m.updateViewportContent()
	m.vp.SetYOffset(12)

	newTask := makeTask("new-task", "New Task")
	updated, _ := m.Update(taskDetailMsg{
		Task:            &newTask,
		Comments:        nil,
		BackState:       stateTasks,
		PreserveHistory: false,
	})
	got := updated.(*AppModel)

	if got.selectedTask.ID != newTask.ID {
		t.Fatalf("selected task = %q, want %q", got.selectedTask.ID, newTask.ID)
	}
	if got.vp.YOffset != 0 {
		t.Fatalf("viewport y offset = %d, want 0", got.vp.YOffset)
	}
	if got.prevState != stateTasks {
		t.Fatalf("prevState = %v, want %v", got.prevState, stateTasks)
	}
	if got.detailBackState != stateTasks {
		t.Fatalf("detailBackState = %v, want %v", got.detailBackState, stateTasks)
	}
	if !strings.Contains(got.vp.View(), newTask.Name) {
		t.Fatalf("viewport does not show new task name: %q", got.vp.View())
	}
}

func TestTaskDetailMsgPreservesHistoryWhenRequested(t *testing.T) {
	m := newTestModel(t)
	m.selectedTask = makeTask("child-task", "Child Task")
	m.taskHistory = []clickup.Task{makeTask("parent-task", "Parent Task")}

	targetTask := makeTask("next-task", "Next Task")
	updated, _ := m.Update(taskDetailMsg{
		Task:            &targetTask,
		Comments:        nil,
		BackState:       stateTaskDetail,
		PreserveHistory: true,
	})
	got := updated.(*AppModel)

	if len(got.taskHistory) != 1 {
		t.Fatalf("taskHistory len = %d, want 1", len(got.taskHistory))
	}
	if got.taskHistory[0].ID != "parent-task" {
		t.Fatalf("taskHistory[0] = %q, want %q", got.taskHistory[0].ID, "parent-task")
	}
}

func TestTaskCreatedMsgShowsCreatedTaskDetail(t *testing.T) {
	m := newTestModel(t)

	staleTask := makeTask("stale-task", "Stale Task")
	staleTask.Desc = strings.Repeat("stale line\n", 80)
	m.selectedTask = staleTask
	m.selectedComments = []clickup.Comment{{ID: "old-comment", CommentText: "stale"}}
	m.state = stateTasks
	m.prevState = stateTasks
	m.teamMembersTaskID = staleTask.ID
	m.updateViewportContent()
	m.vp.SetYOffset(15)

	createdTask := makeTask("created-task", "Created Task")
	updated, _ := m.Update(taskCreatedMsg{
		Task:      &createdTask,
		Tasks:     []clickup.Task{createdTask},
		Comments:  nil,
		BackState: stateTasks,
	})
	got := updated.(*AppModel)

	if got.state != stateTaskDetail {
		t.Fatalf("state = %v, want %v", got.state, stateTaskDetail)
	}
	if got.selectedTask.ID != createdTask.ID {
		t.Fatalf("selected task = %q, want %q", got.selectedTask.ID, createdTask.ID)
	}
	if len(got.selectedComments) != 0 {
		t.Fatalf("selected comments = %d, want 0", len(got.selectedComments))
	}
	if len(got.allTasks) != 1 || got.allTasks[0].ID != createdTask.ID {
		t.Fatalf("allTasks = %#v, want created task only", got.allTasks)
	}
	if got.vp.YOffset != 0 {
		t.Fatalf("viewport y offset = %d, want 0", got.vp.YOffset)
	}
	if !strings.Contains(got.vp.View(), createdTask.Name) {
		t.Fatalf("viewport does not show created task name: %q", got.vp.View())
	}
}

func TestCommandEnterExecutesTypedChecklistAddNameThatMatchesSuggestion(t *testing.T) {
	m := newTestModel(t)
	m.state = stateCommand
	m.prevState = stateTaskDetail
	m.selectedTask = makeTask("task-1", "Task")
	m.updateCommandSuggestions()
	m.cmdInput.SetValue("/checklist add checklist")
	m.filterSuggestions()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*AppModel)

	if got.loadingMsg != "Creating checklist..." {
		t.Fatalf("loadingMsg = %q, want %q", got.loadingMsg, "Creating checklist...")
	}
	if got.state != stateTaskDetail {
		t.Fatalf("state = %v, want %v", got.state, stateTaskDetail)
	}
}

func TestTaskDetailParentShortcutQueuesParentFetch(t *testing.T) {
	m := newTestModel(t)
	parentID := "parent-task"
	m.state = stateTaskDetail
	m.selectedTeam = "team-1"
	m.selectedTask = makeTask("child-task", "Child Task")
	m.selectedTask.Parent = &parentID

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(*AppModel)

	if !got.loading {
		t.Fatalf("loading = %v, want true", got.loading)
	}
	if got.loadingMsg != "Fetching parent task..." {
		t.Fatalf("loadingMsg = %q, want %q", got.loadingMsg, "Fetching parent task...")
	}
	if len(got.taskHistory) != 1 || got.taskHistory[0].ID != "child-task" {
		t.Fatalf("taskHistory = %#v, want child task in history", got.taskHistory)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want fetch command")
	}
}

func TestTaskDetailParentShortcutReplacesImmediateParentHistoryEntry(t *testing.T) {
	m := newTestModel(t)
	parentID := "parent-task"
	m.state = stateTaskDetail
	m.selectedTeam = "team-1"
	m.selectedTask = makeTask("child-task", "Child Task")
	m.selectedTask.Parent = &parentID
	m.taskHistory = []clickup.Task{
		makeTask("grandparent-task", "Grandparent Task"),
		makeTask("parent-task", "Parent Task"),
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(*AppModel)

	if len(got.taskHistory) != 2 {
		t.Fatalf("taskHistory len = %d, want 2", len(got.taskHistory))
	}
	if got.taskHistory[0].ID != "grandparent-task" {
		t.Fatalf("taskHistory[0] = %q, want %q", got.taskHistory[0].ID, "grandparent-task")
	}
	if got.taskHistory[1].ID != "child-task" {
		t.Fatalf("taskHistory[1] = %q, want %q", got.taskHistory[1].ID, "child-task")
	}
}

func TestTaskDetailShareShortcutCopiesTaskURL(t *testing.T) {
	m := newTestModel(t)
	m.state = stateTaskDetail
	m.selectedTask = makeTask("task-1", "Task")
	m.selectedTask.URL = "https://app.clickup.com/t/abc123"
	m.checklistViewItems = []checklistViewItem{
		{
			itemType: checklistTypeHeader,
			checklist: clickup.Checklist{
				Name: "Should not be copied",
			},
		},
	}
	m.checklistSelectedIdx = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(*AppModel)

	if got.popupMsg != "Copied URL to clipboard" {
		t.Fatalf("popupMsg = %q, want %q", got.popupMsg, "Copied URL to clipboard")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want clear popup command")
	}
}

func TestCommandEnterExecutesTypedChecklistAddNameThatMatchesOtherCommand(t *testing.T) {
	m := newTestModel(t)
	m.state = stateCommand
	m.prevState = stateTaskDetail
	m.selectedTask = makeTask("task-1", "Task")
	m.updateCommandSuggestions()
	m.cmdInput.SetValue("/checklist add list")
	m.filterSuggestions()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*AppModel)

	if got.loadingMsg != "Creating checklist..." {
		t.Fatalf("loadingMsg = %q, want %q", got.loadingMsg, "Creating checklist...")
	}
	if got.state != stateTaskDetail {
		t.Fatalf("state = %v, want %v", got.state, stateTaskDetail)
	}
}

func TestCommandEnterStatusFromTaskListUsesCursorTask(t *testing.T) {
	m := newTestModel(t)
	m.state = stateCommand
	m.prevState = stateTasks
	m.selectedTeam = "team-1"
	m.selectedList = "list-1"
	m.allTasks = []clickup.Task{
		makeTask("task-1", "Task 1"),
		makeTask("task-2", "Task 2"),
	}
	m.applyTaskFilter("")
	m.activeList = &m.tasksList
	m.tasksList.Select(1)
	m.cmdInput.SetValue("/status done")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*AppModel)

	if got.loadingMsg != "Updating status..." {
		t.Fatalf("loadingMsg = %q, want %q", got.loadingMsg, "Updating status...")
	}
	if got.selectedTask.ID != "task-2" {
		t.Fatalf("selectedTask.ID = %q, want %q", got.selectedTask.ID, "task-2")
	}
	if got.state != stateTasks {
		t.Fatalf("state = %v, want %v", got.state, stateTasks)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want status update command")
	}
}

func TestStatusUpdatedMsgReselectsUpdatedTaskInTaskList(t *testing.T) {
	m := newTestModel(t)
	m.state = stateTasks
	m.prevState = stateTasks
	m.allTasks = []clickup.Task{
		makeTask("task-1", "Task 1"),
		makeTask("task-2", "Task 2"),
	}
	m.applyTaskFilter("")
	m.activeList = &m.tasksList
	m.tasksList.Select(0)

	updatedTask := makeTask("task-2", "Task 2")
	updatedTask.Status.Status = "done"

	updated, _ := m.Update(statusUpdatedMsg{
		Task:     &updatedTask,
		Tasks:    []clickup.Task{makeTask("task-1", "Task 1"), updatedTask},
		Comments: nil,
	})
	got := updated.(*AppModel)

	selected, ok := got.tasksList.SelectedItem().(taskItem)
	if !ok {
		t.Fatal("selected item is not a task")
	}
	if selected.ID != "task-2" {
		t.Fatalf("selected item id = %q, want %q", selected.ID, "task-2")
	}
	if got.selectedTask.Status.Status != "done" {
		t.Fatalf("selectedTask status = %q, want %q", got.selectedTask.Status.Status, "done")
	}
}

func TestTaskFieldsUpdatedMsgReselectsUpdatedTaskInTaskList(t *testing.T) {
	m := newTestModel(t)
	m.state = stateTaskDetail
	m.prevState = stateTaskDetail
	m.allTasks = []clickup.Task{
		makeTask("task-1", "Task 1"),
		makeTask("task-2", "Task 2"),
	}
	m.applyTaskFilter("")
	m.activeList = &m.tasksList
	m.tasksList.Select(0)

	updatedTask := makeTask("task-2", "Task 2")
	points := 5.0
	updatedTask.Points = &points

	updated, _ := m.Update(taskFieldsUpdatedMsg{
		Task:     &updatedTask,
		Tasks:    []clickup.Task{makeTask("task-1", "Task 1"), updatedTask},
		Comments: nil,
		Popup:    "Updated points to 5",
	})
	got := updated.(*AppModel)

	selected, ok := got.tasksList.SelectedItem().(taskItem)
	if !ok {
		t.Fatal("selected item is not a task")
	}
	if selected.ID != "task-2" {
		t.Fatalf("selected item id = %q, want %q", selected.ID, "task-2")
	}
	if got.selectedTask.Points == nil || *got.selectedTask.Points != 5 {
		t.Fatalf("selectedTask points = %#v, want 5", got.selectedTask.Points)
	}
	if got.popupMsg != "Updated points to 5" {
		t.Fatalf("popupMsg = %q, want %q", got.popupMsg, "Updated points to 5")
	}
}

func TestCommandSuggestionsIncludeStatusInTaskList(t *testing.T) {
	m := newTestModel(t)
	m.prevState = stateTasks
	m.selectedSpace = "space-1"
	m.allSpaces = []clickup.Space{
		{
			ID: "space-1",
			Statuses: []clickup.TaskStatus{
				{Status: "open"},
				{Status: "done"},
			},
		},
	}
	m.allTasks = []clickup.Task{
		makeTask("task-1", "Task 1"),
		makeTask("task-2", "Task 2"),
	}

	m.updateCommandSuggestions()

	if !hasSuggestion(m.suggestions, "/status ") {
		t.Fatal("missing /status suggestion in task list command palette")
	}
	if !hasSuggestion(m.suggestions, "/status done") {
		t.Fatal("missing concrete /status done suggestion in task list command palette")
	}
}

func TestApplyTaskDetailSelectsNewChecklistItemAfterRefresh(t *testing.T) {
	m := newTestModel(t)
	m.state = stateChecklist
	m.checklistSelectedIdx = 0
	m.checklistSelection = &checklistSelectionTarget{
		checklistID:    "checklist-1",
		selectLastItem: true,
	}

	task := makeTask("task-1", "Task")
	task.Checklists = []clickup.Checklist{
		{
			ID:   "checklist-1",
			Name: "Checklist",
			Items: []clickup.ChecklistItem{
				{ID: "item-1", Name: "First", DateCreated: "100"},
				{ID: "item-2", Name: "Second", DateCreated: "200"},
			},
		},
	}

	updated, _ := m.applyTaskDetail(&task, nil, stateTaskDetail, false)
	got := updated.(*AppModel)

	if got.checklistSelectedIdx != 2 {
		t.Fatalf("checklistSelectedIdx = %d, want 2", got.checklistSelectedIdx)
	}
	if got.checklistViewItems[got.checklistSelectedIdx].item.Name != "Second" {
		t.Fatalf("selected item = %q, want %q", got.checklistViewItems[got.checklistSelectedIdx].item.Name, "Second")
	}
	if got.checklistSelection != nil {
		t.Fatalf("checklistSelection = %#v, want nil", got.checklistSelection)
	}
}

func TestApplyTaskDetailSelectsNewChecklistHeaderAfterRefresh(t *testing.T) {
	m := newTestModel(t)
	m.state = stateChecklist
	m.checklistSelectedIdx = 0
	m.checklistSelection = &checklistSelectionTarget{selectLastChecklist: true}

	task := makeTask("task-1", "Task")
	task.Checklists = []clickup.Checklist{
		{ID: "checklist-1", Name: "First"},
		{ID: "checklist-2", Name: "Second"},
	}

	updated, _ := m.applyTaskDetail(&task, nil, stateTaskDetail, false)
	got := updated.(*AppModel)

	if got.checklistSelectedIdx != 1 {
		t.Fatalf("checklistSelectedIdx = %d, want 1", got.checklistSelectedIdx)
	}
	if got.checklistViewItems[got.checklistSelectedIdx].checklist.Name != "Second" {
		t.Fatalf("selected checklist = %q, want %q", got.checklistViewItems[got.checklistSelectedIdx].checklist.Name, "Second")
	}
	if got.checklistSelection != nil {
		t.Fatalf("checklistSelection = %#v, want nil", got.checklistSelection)
	}
}
