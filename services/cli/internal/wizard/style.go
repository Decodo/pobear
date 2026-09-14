package wizard

import "github.com/charmbracelet/lipgloss"

var (
	borderColor = lipgloss.Color("39")

	borderStyle   = lipgloss.NewStyle().Foreground(borderColor)
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	bearAccent    = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
)
