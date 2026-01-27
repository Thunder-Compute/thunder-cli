package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderPortsHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                               PORTS COMMAND                                 │
│                   Manage HTTP port forwarding for instances                 │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports <command>"))
	output.WriteString("\n\n")

	// Commands Section
	output.WriteString(SectionStyle.Render("● COMMANDS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("list"))
	output.WriteString("      ")
	output.WriteString(DescStyle.Render("List forwarded ports for all instances"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("forward"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Forward HTTP ports for an instance"))
	output.WriteString("\n\n")

	// Aliases Section
	output.WriteString(SectionStyle.Render("● ALIASES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr port    → tnr ports"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports ls  → tnr ports list"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports fwd → tnr ports forward"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# List all forwarded ports"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports list"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Forward ports interactively"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add ports to instance 1"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward 1 --add 8080,3000"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Remove a port from instance 1"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward 1 --remove 443"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add and remove ports in one command"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports fwd 1 --add 8080 --remove 443"))
	output.WriteString("\n\n")

	// Notes Section
	output.WriteString(SectionStyle.Render("● NOTES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Port 22 is reserved for SSH and cannot be forwarded"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Valid port range: 1-65535"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Forwarded ports are accessible at https://<instance-ip>:<port>.thundercompute.net"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}

func RenderPortsForwardHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                            PORTS FORWARD COMMAND                            │
│                      Forward HTTP ports for an instance                     │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports forward [instance] [flags]"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--add"))
	output.WriteString("      ")
	output.WriteString(DescStyle.Render("Ports to add (comma-separated)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--remove"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Ports to remove (comma-separated)"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode (select instance and ports)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add ports 8080 and 3000 to instance 1"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward 1 --add 8080,3000"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Remove port 443 from instance 1"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ports forward 1 --remove 443"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}

func RenderPortsListHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                             PORTS LIST COMMAND                              │
│                   List forwarded ports for all instances                    │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports list"))
	output.WriteString("\n\n")

	// Aliases Section
	output.WriteString(SectionStyle.Render("● ALIASES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ports ls"))
	output.WriteString("\n\n")

	// Example Output Section
	output.WriteString(SectionStyle.Render("● EXAMPLE OUTPUT"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Instance   Status     IP              Forwarded Ports"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("--------   -------    --------------  ----------------"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("1          RUNNING    10.0.0.5        8080, 3000, 8443"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("2          RUNNING    10.0.0.12       (none)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("3          STOPPED    -               5000"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
