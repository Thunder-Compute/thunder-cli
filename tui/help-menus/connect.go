/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// RenderConnectHelp renders the custom help for the connect command
func RenderConnectHelp(cmd *cobra.Command) {
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
	output.WriteString("\n\n")

	// Description
	output.WriteString(DescStyle.Render(cmd.Long))
	output.WriteString("\n\n\n")

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
	output.WriteString(ExampleStyle.Render("# Connect to instance with ID 'abc123'"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("tnr connect abc123"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive instance selection"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("tnr connect"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Connect with port forwarding"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("tnr connect abc123 --tunnel 8080:80 --tunnel 3000:3000"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Connect with debug mode"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("tnr connect abc123 --debug"))
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

	// What happens section
	output.WriteString(SectionStyle.Render("● WHAT HAPPENS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Setup"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("SSH keys generated and configured"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Config"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("SSH config updated with instance details"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Connect"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("SSH connection established"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Ready"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Instance ready for development"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Reconnect"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use 'ssh tnr-{instance_id}' after initial setup"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Port Forward"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use --tunnel for local port forwarding"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Debug"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use --debug for verbose connection logs"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
