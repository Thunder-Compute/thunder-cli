package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSnapshotHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := HelpHeader("SNAPSHOT COMMAND", "Manage Thunder Compute snapshots")

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr snapshot <command>"))
	output.WriteString("\n\n")

	// Commands Section
	output.WriteString(SectionStyle.Render("● COMMANDS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("create"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Create a snapshot from a running instance"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("list"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("List all snapshots"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("delete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Delete a snapshot"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Run 'tnr snapshot <command> --help' for details on a specific command."))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
