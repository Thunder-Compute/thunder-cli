package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderCreateHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                CREATE COMMAND                               │
│                    Create new Thunder Compute GPU instances                 │
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
	output.WriteString(DescStyle.Render("tnr create"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Prototyping"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr create --mode prototyping --gpu {a6000|a100} --vcpus {4|8|16|32|64} --template {base|comfy-ui|ollama} --disk-size-gb {100-400}"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Production"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr create --mode production --num-gpus {1|2|4|8} --template {base|comfy-ui|ollama} --disk-size-gb {100-1000}"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Interactive mode with step-by-step wizard"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Prototyping instance (lowest cost)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode prototyping --gpu a6000 --vcpus 8 --template base --disk-size-gb 100"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Production instance"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode production --gpu a100 --num-gpus 2 --template base --disk-size-gb 500"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Attach a saved SSH key during creation"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode prototyping --gpu a6000 --vcpus 4 --template base --ssh-key my-key"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--mode"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Instance mode: prototyping or production"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--gpu"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("GPU type (prototyping: a6000 or a100, production: a100 or h100)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--num-gpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Number of GPUs (production only): 1, 2, 4, or 8"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--vcpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("CPU cores (prototyping only): 4, 8, 16, 32, or 64, RAM: 8GB per vCPU. Production: 18 per GPU, RAM: 90GB per GPU"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--template"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("base, comfy-ui, ollama"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--disk-size-gb"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Disk storage in GB: 100-400 (prototyping), 100-1000 (production)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--ssh-key"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("[Optional] Name of an external SSH key to attach (see 'tnr ssh-keys --help')"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
