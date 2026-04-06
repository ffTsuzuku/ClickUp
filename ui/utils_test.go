package ui

import (
	"testing"

	"github.com/tsuzuku/clickup-tui/clickup"
)

func TestFlattenChecklistsUsesParentHierarchyOnly(t *testing.T) {
	parentID := "item-1"
	incorrectChildID := "item-3"

	m := &AppModel{
		selectedTask: clickup.Task{
			Checklists: []clickup.Checklist{
				{
					ID:   "checklist-1",
					Name: "Checklist",
					Items: []clickup.ChecklistItem{
						{
							ID:          "item-1",
							Name:        "first",
							DateCreated: "100",
							Children:    []clickup.ChecklistItem{{ID: incorrectChildID, Name: "third"}},
						},
						{
							ID:          "item-2",
							Name:        "second",
							DateCreated: "200",
							Parent:      &parentID,
							Resolved:    true,
						},
						{
							ID:          incorrectChildID,
							Name:        "third",
							DateCreated: "300",
							Resolved:    true,
						},
					},
				},
			},
		},
	}

	m.flattenChecklists()

	if len(m.checklistViewItems) != 4 {
		t.Fatalf("expected 4 view items, got %d", len(m.checklistViewItems))
	}

	gotNames := []string{
		m.checklistViewItems[1].item.Name,
		m.checklistViewItems[2].item.Name,
		m.checklistViewItems[3].item.Name,
	}
	wantNames := []string{"first", "second", "third"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("unexpected item order at %d: got %q want %q", i, gotNames[i], wantNames[i])
		}
	}

	if m.checklistViewItems[1].depth != 0 {
		t.Fatalf("expected first item depth 0, got %d", m.checklistViewItems[1].depth)
	}
	if m.checklistViewItems[2].depth != 1 {
		t.Fatalf("expected second item depth 1, got %d", m.checklistViewItems[2].depth)
	}
	if m.checklistViewItems[3].depth != 0 {
		t.Fatalf("expected third item depth 0, got %d", m.checklistViewItems[3].depth)
	}
}

func TestGetSubtasksSortsByCreationTimeAscending(t *testing.T) {
	parentID := "parent-1"

	m := &AppModel{
		allTasks: []clickup.Task{
			{ID: "task-3", Name: "third", Parent: &parentID, DateCreated: "300"},
			{ID: "task-2", Name: "second", Parent: &parentID, DateCreated: "200"},
			{ID: "task-1", Name: "first", Parent: &parentID, DateCreated: "100"},
			{ID: "other", Name: "other"},
		},
	}

	got := m.getSubtasks(parentID)
	if len(got) != 3 {
		t.Fatalf("expected 3 subtasks, got %d", len(got))
	}

	wantIDs := []string{"task-1", "task-2", "task-3"}
	for i := range wantIDs {
		if got[i].ID != wantIDs[i] {
			t.Fatalf("unexpected subtask order at %d: got %q want %q", i, got[i].ID, wantIDs[i])
		}
	}
}

func TestFlattenChecklistsSortsByCreationTimeWithinHierarchy(t *testing.T) {
	parentID := "item-1"

	m := &AppModel{
		selectedTask: clickup.Task{
			Checklists: []clickup.Checklist{
				{
					ID:   "checklist-1",
					Name: "Checklist",
					Items: []clickup.ChecklistItem{
						{ID: "item-3", Name: "third", DateCreated: "300"},
						{ID: "item-2", Name: "second", DateCreated: "200", Parent: &parentID},
						{ID: "item-1", Name: "first", DateCreated: "100"},
					},
				},
			},
		},
	}

	m.flattenChecklists()

	gotNames := []string{
		m.checklistViewItems[1].item.Name,
		m.checklistViewItems[2].item.Name,
		m.checklistViewItems[3].item.Name,
	}
	wantNames := []string{"first", "second", "third"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("unexpected item order at %d: got %q want %q", i, gotNames[i], wantNames[i])
		}
	}

	if m.checklistViewItems[1].depth != 0 {
		t.Fatalf("expected first item depth 0, got %d", m.checklistViewItems[1].depth)
	}
	if m.checklistViewItems[2].depth != 1 {
		t.Fatalf("expected second item depth 1, got %d", m.checklistViewItems[2].depth)
	}
	if m.checklistViewItems[3].depth != 0 {
		t.Fatalf("expected third item depth 0, got %d", m.checklistViewItems[3].depth)
	}
}

func TestFlattenChecklistsRendersChildOnlyItemsFromChildrenPayload(t *testing.T) {
	m := &AppModel{
		selectedTask: clickup.Task{
			Checklists: []clickup.Checklist{
				{
					ID:   "checklist-1",
					Name: "Checklist",
					Items: []clickup.ChecklistItem{
						{
							ID:          "item-1",
							Name:        "first",
							DateCreated: "100",
							Children: []clickup.ChecklistItem{
								{
									ID:          "item-2",
									Name:        "second",
									DateCreated: "200",
								},
							},
						},
						{
							ID:          "item-3",
							Name:        "third",
							DateCreated: "300",
						},
					},
				},
			},
		},
	}

	m.flattenChecklists()

	if len(m.checklistViewItems) != 4 {
		t.Fatalf("expected 4 view items, got %d", len(m.checklistViewItems))
	}

	gotNames := []string{
		m.checklistViewItems[1].item.Name,
		m.checklistViewItems[2].item.Name,
		m.checklistViewItems[3].item.Name,
	}
	wantNames := []string{"first", "second", "third"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("unexpected item order at %d: got %q want %q", i, gotNames[i], wantNames[i])
		}
	}

	if m.checklistViewItems[1].depth != 0 {
		t.Fatalf("expected first item depth 0, got %d", m.checklistViewItems[1].depth)
	}
	if m.checklistViewItems[2].depth != 1 {
		t.Fatalf("expected second item depth 1, got %d", m.checklistViewItems[2].depth)
	}
	if m.checklistViewItems[3].depth != 0 {
		t.Fatalf("expected third item depth 0, got %d", m.checklistViewItems[3].depth)
	}
}
