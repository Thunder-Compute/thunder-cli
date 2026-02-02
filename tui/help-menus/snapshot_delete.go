package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSnapshotDeleteHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                         SNAPSHOT DELETE COMMAND                             │
│                        Delete a snapshot permanently                        │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("        ")
	output.WriteString(DescStyle.Render("tnr snapshot delete"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("tnr snapshot delete <snapshot_name>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode with snapshot selection"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr snapshot delete"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Delete specific snapshot by name"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr snapshot delete my-snapshot"))
	output.WriteString("\n\n")

	// Important Notes Section
	output.WriteString(SectionStyle.Render("● IMPORTANT"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• This action is IRREVERSIBLE"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• The snapshot will be permanently destroyed"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Confirmation is always required before deletion"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• You can delete snapshots in any status (READY, CREATING, FAILED)"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
