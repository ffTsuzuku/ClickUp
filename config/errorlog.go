package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ErrorLogEntry struct {
	Time      time.Time
	Context   string
	Message   string
	Profile   string
	State     string
	Workspace string
	Space     string
	List      string
	TaskID    string
	TaskName  string
}

func errorLogPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "error.log"), nil
}

func AppendErrorLog(entry ErrorLogEntry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}

	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := errorLogPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(formatErrorLogEntry(entry))
	return err
}

func formatErrorLogEntry(entry ErrorLogEntry) string {
	var b strings.Builder
	timestamp := entry.Time.Format("2006-01-02 15:04:05 MST")

	writeField := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			value = "-"
		}
		lines := strings.Split(value, "\n")
		b.WriteString(fmt.Sprintf("  %-11s %s\n", label+":", lines[0]))
		for _, line := range lines[1:] {
			b.WriteString(fmt.Sprintf("  %-11s %s\n", "", line))
		}
	}

	b.WriteString("╭──────────────────────────────────────────────────────────────╮\n")
	b.WriteString("│ TOTUI Error Report                                           │\n")
	b.WriteString("╰──────────────────────────────────────────────────────────────╯\n")
	writeField("Time", timestamp)
	writeField("Context", entry.Context)
	writeField("Message", entry.Message)
	writeField("Profile", entry.Profile)
	writeField("State", entry.State)
	writeField("Workspace", entry.Workspace)
	writeField("Space", entry.Space)
	writeField("List", entry.List)
	writeField("Task ID", entry.TaskID)
	writeField("Task Name", entry.TaskName)
	b.WriteString(strings.Repeat("─", 64) + "\n\n")

	return b.String()
}
