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
	output.WriteString("\n\n")

	// Description
	output.WriteString(DescStyle.Render(cmd.Long))
	output.WriteString("\n\n\n")

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
	output.WriteString(DescStyle.Render("tnr create --mode prototyping --gpu t4 --vcpus 8"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Production"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr create --mode production --num-gpus 2"))
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
	output.WriteString(CommandTextStyle.Render("tnr create --mode prototyping --gpu t4 --vcpus 8 --template base --disk-size-gb 100"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Production instance (highest stability)"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode production --num-gpus 2 --template base --disk-size-gb 500"))
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
	output.WriteString(DescStyle.Render("GPU type (prototyping: t4 or a100, production: a100 or h100)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--num-gpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Number of GPUs (production only): 1, 2, or 4"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--vcpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("CPU cores (prototyping only): 4, 8, 16, or 32"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--template"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("OS template key or name"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--disk-size-gb"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Disk storage in GB (100-1000)"))
	output.WriteString(" ")
	output.WriteString(ExampleStyle.Render("(default: 100)"))
	output.WriteString("\n\n")

	// Instance Modes Section
	output.WriteString(SectionStyle.Render("● INSTANCE MODES"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Prototyping"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Default, lowest cost mode"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• GPU: T4 or A100"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• GPU Count: Exactly 1 GPU"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• vCPUs: 4, 8, 16, or 32 (8GB RAM per vCPU)"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Use Case: Development and testing"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Production"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Highest stability mode"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• GPU: A100 or H100"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• GPU Count: 1, 2, or 4 GPUs"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• vCPUs: Fixed at 18 per GPU (144GB RAM per GPU)"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(DescStyle.Render("• Use Case: Long-running production workloads"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Interactive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use interactive mode for guided setup"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Cost"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Prototyping mode offers lowest cost options"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Performance"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Production mode provides highest stability"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
