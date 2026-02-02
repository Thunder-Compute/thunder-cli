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
	output.WriteString(CommandTextStyle.Render("tnr completion bash"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate zsh completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr completion zsh"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate fish completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr completion fish"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Generate PowerShell completion script"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr completion powershell"))
	output.WriteString("\n\n")

	// Supported Shells Section
	output.WriteString(SectionStyle.Render("● SUPPORTED SHELLS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Bash"))
	output.WriteString("         ")
	output.WriteString(DescStyle.Render("Add to ~/.bashrc or ~/.bash_profile"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Zsh"))
	output.WriteString("          ")
	output.WriteString(DescStyle.Render("Add to ~/.zshrc"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Fish"))
	output.WriteString("         ")
	output.WriteString(DescStyle.Render("Add to ~/.config/fish/config.fish"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("PowerShell"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Add to PowerShell profile"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
