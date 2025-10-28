/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Padding(1, 0)

	SectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			MarginTop(1)

	CommandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0391ff")).
			Width(20)

	DescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	LinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Underline(true)

	FlagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ff6b35")).
			Width(15)

	ExampleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)
)

func RenderRootHelp(cmd *cobra.Command) {
	var output strings.Builder

	// Get version from cobra command
	version := cmd.Root().Version
	if version == "" {
		version = "1.0.0"
	}
	versionText := "v " + version

	// Calculate centering (77 chars total width inside the box)
	boxWidth := 77
	leftPadding := (boxWidth - len(versionText)) / 2
	rightPadding := boxWidth - len(versionText) - leftPadding

	header := fmt.Sprintf(`
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                         ⚡  THUNDER COMPUTE CLI  ⚡                         │
│%s%s%s│
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`, strings.Repeat(" ", leftPadding), versionText, strings.Repeat(" ", rightPadding))

	output.WriteString(HeaderStyle.Render(header))
	output.WriteString("\n\n")

	// Description
	output.WriteString(DescStyle.Render(cmd.Long))
	output.WriteString("\n\n\n")

	// Quick Start Section
	output.WriteString(SectionStyle.Render("● QUICK START"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1.  Authenticate"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("tnr login"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2.  Create instance"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("tnr create"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3.  Connect SSH"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("tnr connect <id>"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4.  Check status"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("tnr status"))
	output.WriteString("\n\n")

	// Commands Section
	output.WriteString(SectionStyle.Render("● AVAILABLE COMMANDS"))
	output.WriteString("\n\n")

	for _, subcmd := range cmd.Commands() {
		if subcmd.IsAvailableCommand() && subcmd.Name() != "help" {
			output.WriteString("  ")
			output.WriteString(CommandStyle.Render(subcmd.Name()))
			output.WriteString("   ")
			output.WriteString(DescStyle.Render(subcmd.Short))
			output.WriteString("\n")
		}
	}
	output.WriteString("\n")

	// Footer
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Docs"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("https://www.thundercompute.com/docs"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Help"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("use tnr <command> --help"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Completion"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr completion <bash|zsh|fish|powershell>"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
