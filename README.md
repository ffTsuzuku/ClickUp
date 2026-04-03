# ToTUI

Keyboard-first ClickUp client for the terminal, built with Bubble Tea.

This application lets you browse ClickUp workspaces, spaces, folders, lists, and tasks without leaving the shell. The current codebase supports task editing, comments, checklists, attachments, list and space management, saved routing defaults, and multiple ClickUp profiles.

## What It Does

- Browse `Workspace -> Space -> Folder/List -> Task` hierarchies.
- Open tasks by list navigation, global search, or direct ticket ID.
- Create, rename, move, assign, reprioritize, and delete tasks and subtasks.
- Edit task titles and descriptions inline or in your external `$EDITOR`.
- Add, edit, reply to, and delete comments with `@mention` suggestions.
- Manage checklists and checklist items, including nesting and toggling completion.
- Upload, open, download, and share task attachments.
- Create, rename, and delete spaces and lists from inside the TUI.
- Save default workspace/space/folder/list routing and a default assignee filter.
- Switch between multiple ClickUp profiles from one config file.

## Project Layout

- [`main.go`](/Users/tsuzuku/git/anti_gravity/ClickUp/main.go): Bubble Tea entrypoint.
- [`clickup/`](/Users/tsuzuku/git/anti_gravity/ClickUp/clickup): ClickUp API client for teams, spaces, lists, tasks, comments, checklists, attachments, and search.
- [`config/`](/Users/tsuzuku/git/anti_gravity/ClickUp/config): Config loading/saving and local error logging.
- [`ui/`](/Users/tsuzuku/git/anti_gravity/ClickUp/ui): TUI state machine, views, commands, and keyboard handling.

## Requirements

- Go `1.25.6` or newer
- A ClickUp API token

## Build And Run

```bash
go build -o clickup-tui .
./clickup-tui
```

The app launches in the terminal alternate screen.

## Configuration

The app reads and writes config at `~/.config/totui/totui.json`.

The current format supports multiple profiles:

```json
{
  "active_profile": "work",
  "profiles": {
    "work": {
      "clickup_api_key": "pk_...",
      "clickup_user_name": "alice",
      "clickup_team_id": "123",
      "clickup_space_id": "456",
      "clickup_folder_id": "789",
      "clickup_list_id": "101112"
    }
  }
}
```

Notes:

- `clickup_api_key` is required to authenticate.
- The routing IDs are optional saved defaults. If they become stale or inaccessible, the app clears them automatically.
- Legacy flat config fields are still read for backward compatibility.
- The default assignee filter is stored in `clickup_user_name`.

## Navigation And Editing

- `Enter` or `Right`: open the selected item.
- `Esc`, `Left`, or `q`: go back.
- `j` / `k` or arrow keys: move selection.
- `/` or `:`: open command mode with suggestions.
- `r`: refresh the current view.
- `o`: open the highlighted task or attachment target in the browser.

Task detail shortcuts:

- `c`: add a comment.
- `a`: create a subtask.
- `s`: copy the task URL.
- `A`: copy the full task context for AI prompting.
- `L`: open checklist view.
- `C`: open comments view.
- `1`-`9`: jump to visible subtasks.

Editor shortcuts:

- In comment or description editors, `Ctrl+S` saves.
- `Ctrl+E` opens the current content in your external `$EDITOR`.

## Slash Commands

Global and list navigation:

- `/help`
- `/filter <text>`
- `/clear`
- `/ticket <task-id>`
- `/search <query>`
- `/default set`
- `/default user <username>`
- `/default user clear`
- `/default clear`

Search supports structured filters alongside free text:

- `status:<value>`
- `assignee:<value>`
- `title:<value>`
- `id:<value>`

Examples:

```text
/search status:in-progress api
/search assignee:alice title:bug
/ticket ABC-123
```

Profile management:

- `/profile create <name> [api-key]`
- `/profile switch <name>`
- `/profile save <name>`
- `/profile token <api-key>`
- `/profile delete <name>`

Space and list management:

- `/space create <name>`
- `/space rename <name>`
- `/space delete`
- `/list create <name>`
- `/list rename <name>`
- `/list delete`

Task detail commands:

- `/status <value>`
- `/priority urgent|high|normal|low|none`
- `/points <number>`
- `/assign <username>`
- `/move`
- `/share`
- `/delete`
- `/subtask`
- `/edit title`
- `/edit desc`
- `/edit desc externally`
- `/copy title`
- `/copy desc`
- `/copy checklist`
- `/copy all`

Checklist commands:

- `/checklist add <name>`
- `/checklist rename <checklist-number> <name>`
- `/checklist delete <checklist-number>`
- `/checklist item add <checklist-number> <name>`
- `/checklist item rename <checklist-number> <item-number> <name>`
- `/checklist item toggle <checklist-number> <item-number>`
- `/checklist item delete <checklist-number> <item-number>`

Attachment and comment commands:

- `/attach upload [path]`
- `/attach open <attachment-number>`
- `/attach download <attachment-number>`
- `/attach share <attachment-number>`
- `/comment edit <comment-number>`
- `/comment reply <comment-number>`
- `/comment delete <comment-number>`

## Checklist View

When checklist view is open:

- `n`: create a checklist.
- `a`: add an item under the current entry.
- `e` or `Enter`: edit the current checklist or item.
- `r`: rename the current checklist or item.
- `Space`: toggle checklist item completion.
- `Tab` / `Shift+Tab`: indent or outdent checklist items.
- `d`: delete the current item, or confirm checklist deletion on a header.

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles](https://github.com/charmbracelet/bubbles)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- [Glamour](https://github.com/charmbracelet/glamour)
- [atotto/clipboard](https://github.com/atotto/clipboard)

## License

MIT. See [`LICENSE`](/Users/tsuzuku/git/anti_gravity/ClickUp/LICENSE).
