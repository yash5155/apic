package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	pathStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	requiredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
)

// methodStyle colours the HTTP verb the way API docs usually do.
func methodStyle(method string) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("232"))
	switch method {
	case "GET":
		return base.Background(lipgloss.Color("39"))
	case "POST":
		return base.Background(lipgloss.Color("42"))
	case "PUT", "PATCH":
		return base.Background(lipgloss.Color("214"))
	case "DELETE":
		return base.Background(lipgloss.Color("203"))
	default:
		return base.Background(lipgloss.Color("245"))
	}
}

// statusStyle: green for 2xx, orange for 3xx/4xx, red for 5xx.
func statusStyle(code int) lipgloss.Style {
	s := lipgloss.NewStyle().Bold(true)
	switch {
	case code >= 200 && code < 300:
		return s.Foreground(lipgloss.Color("42"))
	case code >= 300 && code < 500:
		return s.Foreground(lipgloss.Color("214"))
	default:
		return s.Foreground(lipgloss.Color("203"))
	}
}
