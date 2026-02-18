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
	output.WriteString(DescStyle.Render("tnr login"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2.  Create instance"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr create"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3.  Connect"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr connect <id>"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4.  Check status"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr status"))
	output.WriteString("\n\n")

	// Commands Section - organized by category
	type category struct {
		name     string
		commands []string
	}
	categories := []category{
		{"INSTANCE OPERATIONS", []string{"create", "connect", "start", "stop", "modify", "delete", "status"}},
		{"UTILITIES", []string{"scp", "ports", "snapshot", "ssh-keys"}},
		{"CONFIGURATION", []string{"login", "logout", "update", "completion"}},
	}

	// Build lookup from command name to cobra command
	cmdMap := make(map[string]*cobra.Command)
	for _, subcmd := range cmd.Commands() {
		if subcmd.IsAvailableCommand() && subcmd.Name() != "help" {
			cmdMap[subcmd.Name()] = subcmd
		}
	}

	output.WriteString(SectionStyle.Render("● COMMANDS"))
	output.WriteString("\n")

	for _, cat := range categories {
		output.WriteString("\n")
		output.WriteString("  ")
		output.WriteString(DescStyle.Render(cat.name))
		output.WriteString("\n")
		for _, name := range cat.commands {
			if subcmd, ok := cmdMap[name]; ok {
				output.WriteString("    ")
				output.WriteString(CommandStyle.Render(subcmd.Name()))
				output.WriteString("   ")
				output.WriteString(DescStyle.Render(subcmd.Short))
				output.WriteString("\n")
			}
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
	output.WriteString(CommandStyle.Render("Version"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr --version"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
