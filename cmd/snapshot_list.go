package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var snapshotListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all snapshots",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSnapshotList(); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	snapshotListCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotListHelp(cmd)
	})

	snapshotCmd.AddCommand(snapshotListCmd)
}

func runSnapshotList() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	// Show busy spinner while fetching snapshots
	busy := tui.NewBusyModel("Fetching snapshots...")
	bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
	busyDone := make(chan struct{})
	go func() {
		_, _ = bp.Run()
		close(busyDone)
	}()

	snapshots, err := client.ListSnapshots()
	bp.Send(tui.BusyDoneMsg{})
	<-busyDone

	if err != nil {
		return fmt.Errorf("failed to fetch snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		fmt.Println(tui.WarningStyle().Render("⚠ No snapshots found. Use 'tnr snapshot create' to create a snapshot."))
		return nil
	}

	// Sort by creation time (oldest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt < snapshots[j].CreatedAt
	})

	// Render table
	renderSnapshotTable(snapshots)

	// Show beta notice
	fmt.Println()
	betaNotice := "ℹ Snapshots are currently in beta. Please share feedback with us on Discord (https://discord.gg/nwuETS9jJK) or by emailing support@thundercompute.com"
	fmt.Println(tui.WarningStyle().Render(betaNotice))

	return nil
}

func renderSnapshotTable(snapshots api.ListSnapshotsResponse) {
	tui.InitCommonStyles(os.Stdout)

	headerStyle := tui.PrimaryTitleStyle().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	readyStyle := tui.SuccessStyle()
	creatingStyle := tui.WarningStyle()
	failedStyle := tui.ErrorStyle()

	colWidths := map[string]int{
		"Name":    30,
		"Status":  12,
		"Size":    10,
		"Created": 22,
	}

	var b strings.Builder

	// Header row
	headers := []string{"Name", "Status", "Size", "Created"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = headerStyle.Width(colWidths[h]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	// Separator row
	separatorRow := make([]string, len(headers))
	for i, h := range headers {
		separatorRow[i] = strings.Repeat("─", colWidths[h]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	// Data rows
	for _, snapshot := range snapshots {
		name := truncate(snapshot.Name, colWidths["Name"])

		// Format status with color
		var statusStyle lipgloss.Style
		switch snapshot.Status {
		case "READY":
			statusStyle = readyStyle
		case "CREATING":
			statusStyle = creatingStyle
		case "FAILED":
			statusStyle = failedStyle
		default:
			statusStyle = lipgloss.NewStyle()
		}
		status := statusStyle.Render(truncate(snapshot.Status, colWidths["Status"]))

		size := truncate(fmt.Sprintf("%d GB", snapshot.MinimumDiskSizeGB), colWidths["Size"])

		// Format creation time
		createdTime := time.Unix(snapshot.CreatedAt, 0)
		created := truncate(createdTime.Format("2006-01-02 15:04:05"), colWidths["Created"])

		row := []string{
			cellStyle.Width(colWidths["Name"]).Render(name),
			cellStyle.Width(colWidths["Status"]).Render(status),
			cellStyle.Width(colWidths["Size"]).Render(size),
			cellStyle.Width(colWidths["Created"]).Render(created),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	fmt.Print(b.String())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
