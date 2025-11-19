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

func RenderDeleteHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                               DELETE COMMAND                                │
│                    Permanently remove Thunder Compute instances             │
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
	output.WriteString(DescStyle.Render("tnr delete"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Direct"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr delete <instance_id>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode - select from a list"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr delete"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Direct deletion with instance ID"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr delete 0"))
	output.WriteString("\n\n")

	// What happens section
	output.WriteString(SectionStyle.Render("● WHAT HAPPENS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Delete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Remove instance from Thunder Compute servers"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Clean SSH"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Remove SSH configuration (~/.ssh/config)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Clean Hosts"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Remove from known hosts (~/.ssh/known_hosts)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Complete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Instance permanently removed"))
	output.WriteString("\n\n")

	// Warning section
	output.WriteString(SectionStyle.Render("● WARNING"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("IRREVERSIBLE"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("This action cannot be undone"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("DATA LOSS"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("All data on the instance will be permanently lost"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("NO RECOVERY"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Deleted instances cannot be restored"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use interactive mode to see all instances"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Backup"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Download important data before deletion"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Confirm"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Double-check instance ID before deletion"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Status"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use 'tnr status' to see available instances"))
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
