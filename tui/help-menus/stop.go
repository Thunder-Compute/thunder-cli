package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderStopHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                STOP COMMAND                                 │
│                     Stop a running Thunder Compute instance                 │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr stop"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Direct"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr stop <instance_id>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode - select from a list"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr stop"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Stop instance by ID"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr stop 0"))
	output.WriteString("\n\n")

	// Notes section
	output.WriteString(SectionStyle.Render("● NOTES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Stopping an instance preserves its data. Use 'tnr start' to resume it later."))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
