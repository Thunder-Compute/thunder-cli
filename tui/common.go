package tui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

var (
	helpStyleTUI    lipgloss.Style
	errorStyleTUI   lipgloss.Style
	warningStyleTUI lipgloss.Style
	successStyle    lipgloss.Style
)

func InitCommonStyles(out io.Writer) {
	r := lipgloss.NewRenderer(out)
	helpStyleTUI = r.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	errorStyleTUI = r.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)
	warningStyleTUI = r.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
	successStyle = r.NewStyle().Foreground(lipgloss.Color("#00D787")).Bold(true)
}
