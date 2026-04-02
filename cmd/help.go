package cmd

import (
	"os"

	termx "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// wrapHelp wraps a custom TUI help renderer so that --json produces structured
// JSON and non-TTY output falls back to Cobra's default plain-text help.
func wrapHelp(render func(cmd *cobra.Command)) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if JSONOutput {
			printJSONHelp(cmd)
			return
		}
		if !termx.IsTerminal(os.Stdout.Fd()) {
			printDefaultHelp(cmd)
			return
		}
		render(cmd)
	}
}

type jsonFlag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
}

type jsonCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
}

type jsonHelp struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Usage       string        `json:"usage"`
	Version     string        `json:"version,omitempty"`
	Commands    []jsonCommand `json:"commands,omitempty"`
	Flags       []jsonFlag    `json:"flags,omitempty"`
}

func printJSONHelp(cmd *cobra.Command) {
	h := jsonHelp{
		Name:        cmd.Name(),
		Description: cmd.Short,
		Usage:       cmd.UseLine(),
	}

	if v := cmd.Root().Version; v != "" && cmd == cmd.Root() {
		h.Version = v
	}

	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() && sub.Name() != "help" {
			h.Commands = append(h.Commands, jsonCommand{
				Name:        sub.Name(),
				Description: sub.Short,
				Usage:       sub.UseLine(),
			})
		}
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		h.Flags = append(h.Flags, jsonFlag{
			Name:        "--" + f.Name,
			Shorthand:   f.Shorthand,
			Description: f.Usage,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
		})
	})

	// Include inherited persistent flags (e.g. --json, --yes).
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		h.Flags = append(h.Flags, jsonFlag{
			Name:        "--" + f.Name,
			Shorthand:   f.Shorthand,
			Description: f.Usage,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
		})
	})

	printJSON(h)
}

// printDefaultHelp renders Cobra's built-in plain-text help by temporarily
// clearing the custom help function.
func printDefaultHelp(cmd *cobra.Command) {
	saved := cmd.HelpFunc()
	cmd.SetHelpFunc(nil)
	cmd.Help() //nolint:errcheck // stdout write failure is non-recoverable
	cmd.SetHelpFunc(saved)
}
