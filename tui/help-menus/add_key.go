package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderAddKeyHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                              ADD-KEY COMMAND                                │
│                  Add your SSH public key to an instance                     │
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
	output.WriteString(DescStyle.Render("tnr add-key"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Direct"))
	output.WriteString("        ")
	output.WriteString(DescStyle.Render("tnr add-key <instance_id>"))
	output.WriteString("\n\n")

	// Description Section
	output.WriteString(SectionStyle.Render("● DESCRIPTION"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Add an existing SSH public key from your machine to a running instance."))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("This allows you to SSH into the instance using your existing key pair,"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("rather than generating a new one each time."))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(DescStyle.Render("Default key path: ~/.ssh/id_rsa.pub"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode - select instance and key file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr add-key"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add key to a specific instance"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr add-key 0"))
	output.WriteString("\n\n")

	// Supported Key Types Section
	output.WriteString(SectionStyle.Render("● SUPPORTED KEY TYPES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• RSA (ssh-rsa)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Ed25519 (ssh-ed25519)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• ECDSA (ecdsa-sha2-nistp256/384/521)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Security Keys (sk-ssh-ed25519, sk-ecdsa)"))
	output.WriteString("\n\n")

	// Notes Section
	output.WriteString(SectionStyle.Render("● NOTES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Instance must be in RUNNING state"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Key is added to ~/.ssh/authorized_keys on the instance"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Use Tab for path autocomplete in the interactive flow"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
