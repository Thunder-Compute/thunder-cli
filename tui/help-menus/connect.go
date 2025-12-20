package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderConnectHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                 CONNECT COMMAND                             │
│                     Establish SSH connection to instances                   │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Basic"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr connect [instance_id]"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr connect"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("With flags"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr connect <id> --tunnel 8080:80"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive instance selection"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr connect"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Connect to instance with ID '0'"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr connect 0"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Connect with port forwarding"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr connect 0 --tunnel 8080 --tunnel 3000"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Connect with debug mode"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr connect 0 --debug"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--tunnel, -t"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Port forwarding (can specify multiple times: -t 8080 -t 3000)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--debug"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Show detailed timing breakdown"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
