package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSnapshotListHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                          SNAPSHOT LIST COMMAND                              │
│                        List all your snapshots                              │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr snapshot list"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# List all snapshots"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr snapshot list"))
	output.WriteString("\n\n")

	// Output Section
	output.WriteString(SectionStyle.Render("● OUTPUT"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("The command displays a table with the following columns:"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Name: Snapshot name"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Status: READY, CREATING, or FAILED"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Size: Minimum disk size required"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Created: Snapshot creation date"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
