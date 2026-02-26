package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderModifyHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                MODIFY COMMAND                               │
│                   Modify Thunder Compute instance configuration             │
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
	output.WriteString(DescStyle.Render("tnr modify                    # Select instance from list"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("             "))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr modify [index|id]         # Use instance index (0,1,2...) or ID"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode - select instance and configure step-by-step"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr modify"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Modify instance directly with flags"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr modify 0 --gpu h100 --disk-size-gb 500"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--mode"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Instance mode"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--gpu"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("GPU type"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--num-gpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Number of GPUs: 1-8 (production), 1-2 for H100 (prototyping)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--vcpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("CPU cores (prototyping only): options vary by GPU type and count. RAM: 8GB per vCPU"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--disk-size-gb"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Disk storage in GB"))
	output.WriteString("\n\n")

	// Important Notes Section
	output.WriteString(SectionStyle.Render("● IMPORTANT NOTES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Instance can be selected by index (0, 1, 2...) or by ID"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Instance must be in RUNNING state to modify"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Modifying an instance will restart it (brief downtime)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(DescStyle.Render("• Disk size cannot be reduced (only increased)"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
