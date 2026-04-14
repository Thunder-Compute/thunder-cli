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

	header := HelpHeader("CREATE COMMAND", "Create new Thunder Compute GPU instances")

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
	output.WriteString(CommandStyle.Render("With flags"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr create --mode <mode> --gpu <gpu> [flags]"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Create a prototyping instance"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode prototyping --gpu a6000 --vcpus 8 --template base --primary-disk 100"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Create a production instance"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr create --mode production --gpu a100 --num-gpus 2 --template base --primary-disk 500"))
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
	output.WriteString(DescStyle.Render("GPU type"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--num-gpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Number of GPUs"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--vcpus"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("CPU cores"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--template"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Template or snapshot name"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--snapshot"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Alias for --template"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--primary-disk"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Disk storage in GB"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--ephemeral-disk"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Ephemeral storage in GB, mounted at /ephemeral (default: 0)"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("--ssh-key"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("[Optional] Name of an external SSH key to attach (see 'tnr ssh-keys --help')"))
	output.WriteString("\n")




	fmt.Fprint(os.Stdout, output.String())
}
