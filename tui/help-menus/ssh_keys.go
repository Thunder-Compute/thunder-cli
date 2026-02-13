package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSSHKeysHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                             SSH KEYS COMMAND                                │
│                     Manage saved SSH public keys                            │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys <command>"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● COMMANDS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("list"))
	output.WriteString("     ")
	output.WriteString(DescStyle.Render("List all saved SSH keys"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("add"))
	output.WriteString("      ")
	output.WriteString(DescStyle.Render("Add an SSH public key"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("delete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Delete an SSH key"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Run 'tnr ssh-keys <command> --help' for details on a specific command."))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
