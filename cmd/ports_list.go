package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// portsListCmd represents the ports list command
var portsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List forwarded ports for all instances",
	Long:    "Display a table of all instances with their forwarded HTTP ports.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPortsList(); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	portsListCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderPortsListHelp(cmd)
	})
	portsCmd.AddCommand(portsListCmd)
}

func runPortsList() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	// Fetch instances with spinner
	busy := tui.NewBusyModel("Fetching instances...")
	bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
	busyDone := make(chan struct{})
	go func() {
		_, _ = bp.Run()
		close(busyDone)
	}()

	instances, err := client.ListInstances()
	bp.Send(tui.BusyDoneMsg{})
	<-busyDone

	if err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	// Initialize styles
	theme.Init(os.Stdout)
	tui.InitCommonStyles(os.Stdout)

	headerStyle := tui.PrimaryTitleStyle().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	runningStyle := tui.SuccessStyle()
	startingStyle := tui.WarningStyle()
	stoppedStyle := lipgloss.NewStyle()
	helpStyle := tui.HelpStyle()
	primaryStyle := tui.PrimaryStyle()

	// Define column widths
	colWidths := map[string]int{
		"ID":     4,
		"UUID":   15,
		"Status": 12,
		"Ports":  30,
	}

	var b strings.Builder

	// Render header
	headers := []string{"ID", "UUID", "Status", "Forwarded Ports"}
	headerRow := make([]string, len(headers))
	headerKeys := []string{"ID", "UUID", "Status", "Ports"}
	for i, h := range headers {
		headerRow[i] = headerStyle.Width(colWidths[headerKeys[i]]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	// Render separator
	separatorRow := make([]string, len(headers))
	for i, key := range headerKeys {
		separatorRow[i] = strings.Repeat("─", colWidths[key]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	// Sort instances by ID
	sortedInstances := make([]api.Instance, len(instances))
	copy(sortedInstances, instances)
	sort.Slice(sortedInstances, func(i, j int) bool {
		return sortedInstances[i].ID < sortedInstances[j].ID
	})

	// Track instances with ports for the help message
	hasPortsConfigured := false

	// Render rows
	for _, inst := range sortedInstances {
		id := truncateStr(inst.ID, colWidths["ID"])
		uuid := truncateStr(inst.UUID, colWidths["UUID"])

		// Format status with color
		var statusStyled string
		switch inst.Status {
		case "RUNNING":
			statusStyled = runningStyle.Render(truncateStr(inst.Status, colWidths["Status"]))
		case "STARTING", "SNAPPING", "RESTORING":
			statusStyled = startingStyle.Render(truncateStr(inst.Status, colWidths["Status"]))
		default:
			statusStyled = stoppedStyle.Render(truncateStr(inst.Status, colWidths["Status"]))
		}

		// Format ports
		ports := "(none)"
		if len(inst.HTTPPorts) > 0 {
			hasPortsConfigured = true
			portStrs := make([]string, len(inst.HTTPPorts))
			for i, p := range inst.HTTPPorts {
				portStrs[i] = fmt.Sprintf("%d", p)
			}
			ports = strings.Join(portStrs, ", ")
		}

		row := []string{
			cellStyle.Width(colWidths["ID"]).Render(id),
			cellStyle.Width(colWidths["UUID"]).Render(uuid),
			cellStyle.Width(colWidths["Status"]).Render(statusStyled),
			cellStyle.Width(colWidths["Ports"]).Render(truncateStr(ports, colWidths["Ports"])),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Add helper message about how to connect
	if hasPortsConfigured {
		b.WriteString(primaryStyle.Render("ℹ Access forwarded ports at:"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  https://<uuid>-<port>.thundercompute.net"))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("Use 'tnr ports forward <id>' to add or remove ports"))
	b.WriteString("\n\n")

	fmt.Print(b.String())
	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
