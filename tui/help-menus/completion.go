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

func RenderCompletionHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                             COMPLETION COMMAND                              │
│                    Generate shell autocompletion scripts                    │
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
	output.WriteString(CommandStyle.Render("Bash"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr completion bash"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Zsh"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr completion zsh"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Fish"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr completion fish"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("PowerShell"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr completion powershell"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate bash completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString("tnr completion bash")
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate zsh completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString("tnr completion zsh")
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate fish completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString("tnr completion fish")
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate PowerShell completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString("tnr completion powershell")
	output.WriteString("\n\n")

	// Shell Support Section
	output.WriteString(SectionStyle.Render("● SUPPORTED SHELLS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Bash"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Most common Linux/macOS shell"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Works on Linux, macOS, and WSL"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Add to ~/.bashrc or ~/.bash_profile"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Zsh"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Modern shell with advanced features"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Default on macOS Catalina+"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Add to ~/.zshrc"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Fish"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("User-friendly shell with syntax highlighting"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Great for beginners"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Add to ~/.config/fish/config.fish"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("PowerShell"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Cross-platform shell for Windows/Linux/macOS"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Works on Windows, Linux, and macOS"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Add to PowerShell profile"))
	output.WriteString("\n\n")

	// Setup Instructions Section
	output.WriteString(SectionStyle.Render("● SETUP INSTRUCTIONS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Generate"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Run: tnr completion <shell>"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Save"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Save output to shell configuration file"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Reload"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Restart terminal or run: source ~/.bashrc"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Test"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Type 'tnr ' and press Tab to see completions"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Auto-complete"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Press Tab to see available commands and flags"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Commands"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Completes all tnr commands and subcommands"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Flags"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Completes command flags and options"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Update"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Regenerate when adding new commands"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
