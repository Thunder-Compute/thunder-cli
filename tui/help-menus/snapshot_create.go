package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSnapshotCreateHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                         SNAPSHOT CREATE COMMAND                             │
│                     Create snapshots of running instances                   │
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
	output.WriteString(DescStyle.Render("tnr snapshot create"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("tnr snapshot create --instance-id <id> --name <name>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode with step-by-step wizard"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr snapshot create"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Create snapshot with flags"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr snapshot create --instance-id 123 --name my-snapshot"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--instance-id"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Instance ID or UUID to create a snapshot from"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--name"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Name for the snapshot (required)"))
	output.WriteString("\n\n")

	// Important Notes Section
	output.WriteString(SectionStyle.Render("● IMPORTANT"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Instance must be in RUNNING state to create a snapshot"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• The instance must remain running during snapshot creation"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Snapshot names must be unique"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
