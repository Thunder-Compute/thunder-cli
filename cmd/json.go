package cmd

import (
	"encoding/json"
	"os"
)

func printJSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data) //nolint:errcheck // stdout write failure is non-recoverable
}
