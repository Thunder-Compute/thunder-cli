package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"os"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

// renderPlainStatusTable prints a plain-text tab-aligned table of instances to stdout.
func renderPlainStatusTable(instances []api.Instance) {
	if len(instances) == 0 {
		fmt.Fprintln(os.Stderr, "No instances found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUUID\tSTATUS\tADDRESS\tMODE\tDISK\tGPU\tvCPUs\tRAM\tTEMPLATE")

	for _, inst := range instances {
		gpu := fmt.Sprintf("%sx%s", inst.NumGPUs, utils.FormatGPUType(inst.GPUType))
		disk := fmt.Sprintf("%dGB", inst.Storage)
		ram := fmt.Sprintf("%sGB", inst.Memory)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			inst.ID,
			inst.UUID,
			inst.Status,
			inst.GetIP(),
			strings.ToLower(inst.Mode),
			disk,
			gpu,
			inst.CPUCores,
			ram,
			inst.Template,
		)
	}
	w.Flush()
}

// renderPlainSnapshotTable prints a plain-text tab-aligned table of snapshots to stdout.
func renderPlainSnapshotTable(snapshots []api.Snapshot) {
	if len(snapshots) == 0 {
		fmt.Fprintln(os.Stderr, "No snapshots found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tDISK_GB")

	for _, snap := range snapshots {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
			snap.ID,
			snap.Name,
			snap.Status,
			snap.MinimumDiskSizeGB,
		)
	}
	w.Flush()
}
