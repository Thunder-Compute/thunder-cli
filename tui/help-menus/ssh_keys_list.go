package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSSHKeysListHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                          SSH KEYS LIST COMMAND                              │
│                        List all saved SSH keys                              │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys list"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● OUTPUT"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Displays a table with the following columns:"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Name: Key name"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Fingerprint: SHA256 fingerprint"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Key Type: Algorithm (e.g. ssh-ed25519)"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Created: Date the key was added"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
