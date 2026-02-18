package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderStartHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                               START COMMAND                                 │
│                    Start a stopped Thunder Compute instance                  │
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
	output.WriteString(DescStyle.Render("tnr start"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Direct"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr start <instance_id>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode - select from a list"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr start"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Start instance by ID"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr start 0"))
	output.WriteString("\n\n")

	// Notes section
	output.WriteString(SectionStyle.Render("● NOTES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Starts a previously stopped instance, restoring it to a running state."))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
