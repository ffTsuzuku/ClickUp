package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsuzuku/clickup-tui/clickup"
	"github.com/tsuzuku/clickup-tui/config"
)

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
