package helpmenus

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func RenderSCPHelp(cmd *cobra.Command) {
	InitHelpStyles(os.Stdout)

	var output strings.Builder

	header := `
╭─────────────────────────────────────────────────────────────────────────────╮
│                                                                             │
│                                SCP COMMAND                                  │
│                    Securely copy files between local and remote             │
│                                                                             │
╰─────────────────────────────────────────────────────────────────────────────╯
	`

	output.WriteString(HeaderStyle.Render(header))

	// Usage Section
	output.WriteString(SectionStyle.Render("● USAGE"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Upload"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr scp local_file <instance_id>:/remote/path"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Download"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr scp <instance_id>:/remote/file ./local/path"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Multiple"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr scp file1 file2 file3 <instance_id>:/dest/"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Recursive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("tnr scp -r ./local_dir/ <instance_id>:/remote/"))
	output.WriteString("\n\n")

	// Examples Section
	output.WriteString(SectionStyle.Render("● EXAMPLES"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Upload a single file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr scp myfile.py 0:/home/ubuntu/"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Download a file"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr scp 0:/home/ubuntu/results.txt ./"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Upload multiple files"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr scp file1.py file2.py config.json 0:/home/ubuntu/"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Upload a directory recursively"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr scp -r ./my-project/ 0:/home/ubuntu/projects/"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(ExampleStyle.Render("# Download a directory"))
	output.WriteString("\n")
	output.WriteString("  ")
	output.WriteString(CommandTextStyle.Render("tnr scp -r 0:/home/ubuntu/data/ ./local-data/"))
	output.WriteString("\n\n")

	// Flags Section
	output.WriteString(SectionStyle.Render("● FLAGS"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(FlagStyle.Render("-r, --recursive"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Recursively copy directories"))
	output.WriteString("\n\n")

	// Path Syntax Section
	output.WriteString(SectionStyle.Render("● PATH SYNTAX"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Remote Paths"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("instance_id:/path/to/file"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(ExampleStyle.Render("Example: 0:/home/ubuntu/myfile.py"))
	output.WriteString("\n\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Local Paths"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Regular file system paths"))
	output.WriteString("\n")
	output.WriteString("    ")
	output.WriteString(ExampleStyle.Render("Examples: ./myfile.py or /tmp/file.txt"))
	output.WriteString("\n\n")

	// What happens section
	output.WriteString(SectionStyle.Render("● WHAT HAPPENS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("1. Parse"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Analyze source and destination paths"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("2. Connect"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Establish secure connection to instance"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("3. Transfer"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Copy files with progress tracking"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("4. Verify"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Confirm successful transfer"))
	output.WriteString("\n\n")

	// Tips Section
	output.WriteString(SectionStyle.Render("● TIPS"))
	output.WriteString("\n\n")
	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Progress"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Shows real-time transfer progress"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Multiple Files"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Upload multiple files in one command"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Directories"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use -r flag for recursive directory copying"))
	output.WriteString("\n")

	output.WriteString("  ")
	output.WriteString(CommandStyle.Render("Instance ID"))
	output.WriteString("   ")
	output.WriteString(DescStyle.Render("Use instance ID from 'tnr status' command"))
	output.WriteString("\n\n")

	fmt.Fprint(os.Stdout, output.String())
}
