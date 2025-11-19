/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderRootHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

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

	// Resources Section
	output.WriteString(SectionStyle.Render("● RESOURCES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Docs"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("https://www.thundercompute.com/docs/cli-reference"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Troubleshooting"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("https://www.thundercompute.com/docs/troubleshooting"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
