# Checklist TUI Mode Design

## Goal

Replace command-based checklist interaction (`/checklist item toggle 1 2`) with a dedicated TUI mode for keyboard-driven, direct manipulation.

## Architecture

Add a new `stateChecklist` mode to the existing Bubble Tea application. The checklist view renders all checklists and items as a flat navigable list with visual grouping by checklist. Context-aware key handling determines whether operations target items or checklist headers.

## Layout

The checklist view displays all checklists and their items in a single scrollable list:

```
┌─────────────────────────────────────────┐
│ ● Shopping List                         │
│   1. [ ] Buy milk                       │
│   2. [x] Call mom                       │
│   3. [ ] Finish report                  │
├─────────────────────────────────────────┤
│ ● Project Tasks                         │
│   1. [ ] Review PR                      │
│   2. [x] Update docs                   │
└─────────────────────────────────────────┘
```

- Checklist headers shown with `●` bullet
- Items shown with number prefix, checkbox marker, and name
- Completed items rendered dimmed with strikethrough
- Selection highlight follows current cursor position

## Navigation

- `↑`/`↓` or `j`/`k`: Move selection up/down
- `Esc` or `q`: Exit checklist mode, return to task detail

## Selection Model

The list contains two item types:

1. **ChecklistHeader** - Represents a checklist, identified by `●` prefix
2. **ChecklistItemRow** - Represents a single item within a checklist

Selection can land on either type. When on a header, `↑`/`↓` moves to next header or first item of current checklist. When on an item, `↑` moves within the checklist; at the first item, `↑` goes to the header.

## Item Operations

| Key | Context | Action |
|-----|---------|--------|
| `Space` | Item selected | Toggle item complete |
| `Enter` | Item selected | Edit item name inline (text input) |
| `a` | Item or header selected | Add new item to current checklist |
| `r` | Item or header selected | Rename (item or checklist) |
| `d` | Item selected | Delete item |
| `e` | Item selected | Edit item name inline (alias for Enter) |
| `n` | Header selected | Create new checklist |

### Checklist-Level Operations

| Key | Context | Action |
|-----|---------|--------|
| `R` | Header selected | Rename checklist (explicit) |
| `D` | Header selected | Delete checklist (with confirmation) |

When on a header, `r` and `d` also apply to the checklist (context-aware).

### Confirmation

Deleting a checklist (`d` on header or `D`) shows a confirmation prompt: "Delete checklist 'X'? [y/N]"

## Add Item Flow

1. Press `a` while on an item or header
2. Inline text input appears at end of current checklist
3. Type item name, `Enter` to create, `Esc` to cancel
4. New item appears at end of checklist, selection moves to it

## Rename Flow

1. Press `r` on item or header
2. Inline text input replaces current name
3. Pre-filled with existing name, cursor at end
4. `Enter` to save, `Esc` to cancel
5. If on header, updates checklist name via `UpdateChecklist`
6. If on item, updates via `UpdateChecklistItem`

## New Checklist Flow

1. Press `n` while on any checklist header
2. Inline text input appears
3. Type name, `Enter` to create, `Esc` to cancel
4. New checklist created via `CreateChecklist`, task detail refreshed

## State Management

New model fields in `AppModel`:

```go
stateChecklist                // new state value
checklistViewItems []checklistViewItem  // flattened view data
checklistSelectedIdx int     // current selection index
checklistEditingItem *checklistViewItem // item being edited
checklistEditInput textinput.Model
checklistPendingDelete string // checklist ID if delete pending
```

New message types:
- `checklistItemUpdatedMsg` - refresh after item toggle/rename
- `checklistCreatedMsg` - refresh after new checklist
- `checklistDeletedMsg` - refresh after delete

## API Calls

| Operation | ClickUp API |
|-----------|-------------|
| Toggle item | `UpdateChecklistItem` with inverted `resolved` |
| Add item | `CreateChecklistItem` |
| Rename item | `UpdateChecklistItem` with new name |
| Delete item | `DeleteChecklistItem` |
| Rename checklist | `UpdateChecklist` |
| Delete checklist | `DeleteChecklist` |
| Create checklist | `CreateChecklist` |

All operations that modify data call `refreshTaskDetailCmd` after success to reload task with updated checklists.

## Exit Behavior

- `Esc` or `q` returns to task detail (`stateTaskDetail`)
- Task detail viewport re-renders with current `selectedTask`

## Help Integration

Checklist mode keys documented in help (`/` command):
```
L     : Open checklist view
↑↓/jk : Navigate items
Space : Toggle item complete
Enter : Edit item name
a     : Add new item
r     : Rename (item or checklist)
d     : Delete item
n     : Create new checklist
R     : Rename checklist
D     : Delete checklist
Esc   : Exit to task detail
```

## Deprecation

Old slash commands (`/checklist item toggle`, etc.) remain functional for backward compatibility but are no longer suggested in command palette. Focus shifts entirely to TUI mode.
