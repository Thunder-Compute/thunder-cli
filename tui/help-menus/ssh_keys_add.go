package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSSHKeysAddHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                           SSH KEYS ADD COMMAND                              │
│                       Add an SSH public key                                 │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("        ")
	output.WriteString(DescStyle.Render("tnr ssh-keys add"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("tnr ssh-keys add --name <name> [--key-file <path> | --key <key>]"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--name"))
	output.WriteString("       ")
	output.WriteString(DescStyle.Render("Name for the SSH key (required for non-interactive)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--key-file"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Path to SSH public key file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--key"))
	output.WriteString("        ")
	output.WriteString(DescStyle.Render("SSH public key string"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode (detects local keys)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys add"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add from a file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys add --name my-key --key-file ~/.ssh/id_ed25519.pub"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
