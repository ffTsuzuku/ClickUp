# 🧊 ClickUp TUI 🧊

> A premium, keyboard-centric Text User Interface for ClickUp, built for speed and visual excellence.

![ClickUp TUI Screenshot](https://raw.githubusercontent.com/tsuzuku/clickup-tui/main/screenshot.png) *(Placeholder: Screenshots can be added here)*

ClickUp TUI is a high-performance terminal application designed to bring your workflow into your favorite environment. Built with the **Bubble Tea** framework, it emphasizes a sleek aesthetic, effortless navigation, and frictionless task management.

---

## ✨ Features

- **🚀 Ultra-Fast Navigation:** Effortlessly drill down from Workspaces -> Spaces -> Lists -> Tasks -> Details.
- **🔍 Powerful Search:** Fuzzy-find anything with `/filter` (assignee, status, title, or ID).
- **📝 Description Mastery:** Edit task descriptions with a built-in multi-line `textarea` or swap to your preferred `$EDITOR` (vim/nano/etc) with a single keypress.
- **🏗️ Task Management:** 
  - Quick-add (`a`/`n`) and delete (`/delete`) tickets.
  - Move tickets between lists using an interactive picker (`/move`).
  - Create subtasks (`t`) and navigate existing ones (`1-9`).
- **💬 Community & Effort:** Seamlessly add comments (`c`) and update story points or status.
- **📎 Clipboard Magic:** Copy ticket URLs instantly with `s` or `/share` to your system clipboard.
- **🛠️ Smart Defaults:** Save your favorite Workspace, Space, List, or Assignee filter to auto-load on startup with `/default set`.
- **🔄 Instant Refresh:** Hit `r` to pull the latest state from the ClickUp API at any time.

---

## 🛠️ Installation

1. Ensure you have **Go 1.21+** installed.
2. Clone the repository:
   ```bash
   git clone https://github.com/tsuzuku/clickup-tui.git
   cd clickup-tui
   ```
3. Build the binary:
   ```bash
   go build -o clickup-tui
   ```

---

## ⚙️ Configuration

The application stores settings in `~/.config/totui/totui.json`. On the first run, it will prompt you for your ClickUp API Token, or you can manually create the file:

```json
{
  "ClickupToken": "your_api_token_here",
  "ClickupTeamID": "12345",
  "ClickupSpaceID": "67890",
  "ClickupListID": "54321",
  "ClickupUserName": "your_username"
}
```

---

## ⌨️ Keybindings & Commands

### Navigation
- `Up/Down/j/k`: Navigate items/text
- `Enter/Right`: Select or view detail
- `Esc/Left`: Go back
- `1-9`: Jump to subtask (while viewing a task)

### Task Actions
- `c`: Add comment
- `e`: Edit description (inline editor)
- `E`: Edit description in external `$EDITOR` (e.g., Vim)
- `t`: Create new subtask
- `s`: Copy link to clipboard
- `r`: Refresh current view

### Global Commands (`/`)
- `/filter <query>`: Fuzzy search the current view
- `/ticket <id>`: Jump directly to any ticket ID (e.g., `/ticket OMNI-123`)
- `/assign <user>`: Change ticket assignee
- `/delete`: Permanently delete a ticket
- `/move`: Pick a destination list to move the ticket
- `/default set`: Save current location as startup default
- `/help`: Detailed command reference

---

## 🎨 Technology Stack

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea):** State & UI management
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss):** Styling & Layout
- **[Bubbles](https://github.com/charmbracelet/bubbles):** High-level TUI components (List, Input, Textarea, Spinner)
- **ClickUp API:** Core backend integration

---

## 📜 License

MIT License. See [LICENSE](LICENSE) for more details.
