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

func RenderLogoutHelp(cmd *cobra.Command) {
	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                LOGOUT COMMAND                               │
│                    Remove Thunder Compute authentication                    │
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
	output.WriteString(CommandStyle.Render("Logout"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr logout"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Log out and remove credentials"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString("tnr logout")
	output.WriteString("\n\n")

	// What happens section
	output.WriteString(SectionStyle.Render("● WHAT HAPPENS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Remove"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Delete saved authentication credentials"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Clean"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Remove config file from ~/.thunder-cli-draft/"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Confirm"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Display logout confirmation message"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Complete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("You're now logged out"))
	output.WriteString("\n\n")

	// What gets removed section
	output.WriteString(SectionStyle.Render("● WHAT GETS REMOVED"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Config File"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("~/.thunder-cli-draft/config.json"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Access Token"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Your Thunder Compute API token"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Refresh Token"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Token used for automatic renewal"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Expiry Info"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Token expiration timestamps"))
	output.WriteString("\n\n")

	// After logout section
	output.WriteString(SectionStyle.Render("● AFTER LOGOUT"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Re-authenticate"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Run 'tnr login' to authenticate again"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("No Access"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("All tnr commands will require re-authentication"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Clean Slate"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Fresh start with new credentials"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Security"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Logout when using shared computers"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Cleanup"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use logout to clear old credentials"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Re-login"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Run 'tnr login' to authenticate again"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Safe"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Logout is safe and reversible"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
