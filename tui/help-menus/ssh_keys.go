package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSSHKeysHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                             SSH KEYS COMMAND                                │
│                     Manage saved SSH public keys                            │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys <command>"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● COMMANDS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("list"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("List all saved SSH keys"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("add"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("Add an SSH public key"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("delete"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("Delete an SSH key"))
	output.WriteString("\n\n")

	// LIST subcommand details
	output.WriteString(SectionStyle.Render("● LIST USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys list"))
	output.WriteString("\n\n")
	output.WriteString("  ")

	// ADD subcommand details
	output.WriteString(SectionStyle.Render("● ADD USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys add"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys add --name <name> [--key-file <path> | --key <key>]"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● ADD FLAGS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--name"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("Name for the SSH key (required for non-interactive)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--key-file"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("Path to SSH public key file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--key"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("SSH public key string"))
	output.WriteString("\n\n")

	// DELETE subcommand details
	output.WriteString(SectionStyle.Render("● DELETE USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys delete"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Non-interactive"))
	output.WriteString("                  ")
	output.WriteString(DescStyle.Render("tnr ssh-keys delete <key_name_or_id>"))
	output.WriteString("\n\n")

	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# List all SSH keys"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys list"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add a key interactively (with local key detection)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys add"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add a key from a file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys add --name my-key --key-file ~/.ssh/id_ed25519.pub"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Add a key from string"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys add --name my-key --key \"ssh-ed25519 AAAA...\""))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Delete a key interactively"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys delete"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Delete a specific key by name"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr ssh-keys delete my-key"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
