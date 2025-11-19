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

func RenderStatusHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                               STATUS COMMAND                                │
│                    List and monitor Thunder Compute instances               │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Monitor"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr status"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("One-time"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr status --no-wait"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Continuous monitoring (default)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr status"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Display status once and exit"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr status --no-wait"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--no-wait"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Display status once and exit without monitoring"))
	output.WriteString("\n\n")

	// What you'll see section
	output.WriteString(SectionStyle.Render("● WHAT YOU'LL SEE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Instance List"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("All your Thunder Compute instances"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Status Info"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Current state: running, stopped, creating, etc."))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Resource Usage"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("GPU, CPU, and memory utilization"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Connection Info"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("SSH connection details and IP addresses"))
	output.WriteString("\n\n")

	// Monitoring section
	output.WriteString(SectionStyle.Render("● MONITORING MODE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Real-time"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Continuously refreshes instance status"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Press 'Q' to quit monitoring"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Auto-refresh"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Updates every few seconds"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Quick Check"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use --no-wait for a quick status snapshot"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Monitoring"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Default mode provides real-time updates"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Exit"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Press 'Q' to quit monitoring mode"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Integration"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Perfect for checking instance health"))
	output.WriteString("\n\n")

	// Resources Section
	output.WriteString(SectionStyle.Render("● RESOURCES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Docs"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("https://www.thundercompute.com/docs/cli-reference"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Troubleshooting"))
	output.WriteString("   ")
	output.WriteString(LinkStyle.Render("https://www.thundercompute.com/docs/troubleshooting"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
