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

func RenderLoginHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                LOGIN COMMAND                                │
│                     Authenticate with Thunder Compute                       │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Browser"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr login"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Token"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr login --token <your_token>"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive browser authentication"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr login"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Direct token authentication"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr login --token abc123xyz789"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--token"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Authenticate directly with a token instead of opening browser"))
	output.WriteString("\n\n")

	// What happens section
	output.WriteString(SectionStyle.Render("● WHAT HAPPENS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Browser"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Opens your default browser to Thunder Compute"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Authenticate"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Complete login in the browser"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Save"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Credentials saved to ~/.thunder/cli_config.json"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Ready"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("You're now authenticated and ready to use tnr"))
	output.WriteString("\n\n")

	// Authentication methods section
	output.WriteString(SectionStyle.Render("● AUTHENTICATION METHODS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Browser Login"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Interactive OAuth flow (recommended)"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Opens Thunder Compute login page"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Secure OAuth2 authentication"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Automatic credential storage"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Token Login"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Direct token authentication"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Use existing API token"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Skip browser interaction"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Useful for automation"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("First Time"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use browser login for easiest setup"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Automation"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use --token flag for scripts"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Security"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Credentials stored securely in home directory"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Logout"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use 'tnr logout' to remove credentials"))
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
