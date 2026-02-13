package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSSHKeysDeleteHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                         SSH KEYS DELETE COMMAND                             │
│                         Delete an SSH key                                   │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("        ")
	output.WriteString(DescStyle.Render("tnr ssh-keys delete"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("tnr ssh-keys delete <key_name_or_id>"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys delete"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Delete by name"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys delete my-key"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
