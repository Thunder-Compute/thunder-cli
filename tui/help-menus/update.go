package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderUpdateHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                               UPDATE COMMAND                                │
│                      Update tnr to the latest version                       │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr update"))
	output.WriteString("\n\n")

	// Description Section
	output.WriteString(SectionStyle.Render("● DESCRIPTION"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Runs an update check and downloads/installs the latest"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("version when an update is available."))
	output.WriteString("\n\n")

	// Package Manager Section
	output.WriteString(SectionStyle.Render("● PACKAGE MANAGER INSTALLS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("If tnr was installed via a package manager, this command"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("will print instructions for updating through your package manager:"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("Homebrew"))
	output.WriteString("   ")
	output.WriteString(CommandTextStyle.Render("brew update && brew upgrade tnr"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("Scoop"))
	output.WriteString("   ")
	output.WriteString(CommandTextStyle.Render("scoop update tnr"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("Winget"))
	output.WriteString("   ")
	output.WriteString(CommandTextStyle.Render("winget upgrade Thunder.tnr"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
