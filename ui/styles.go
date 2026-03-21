package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#7D56F4") // ClickUp Purple
	ColorSecondary = lipgloss.Color("#2ea043") // Green for Done
	ColorText      = lipgloss.Color("#e1e4e8")
	ColorSubtext   = lipgloss.Color("#6e7681")
	ColorBorder    = lipgloss.Color("#30363d")
	ColorError     = lipgloss.Color("#FF4D4D")

	// Styles
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	ListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			MarginRight(2)

	DetailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	StatusTodoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	StatusInProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f1e05a"))
	StatusDoneStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	ColorSecondaryStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	
	SectionHeaderStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	LabelStyle         = lipgloss.NewStyle().Foreground(ColorSubtext)
	BreadcrumbStyle    = lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true)
)
