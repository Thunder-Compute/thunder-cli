package cmd

import (
	"os"

	"github.com/Thunder-Compute/thunder-cli/internal/version"
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

	if cmd == cmd.Root() {
		h.Version = version.BuildVersion
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

	seenFlags := make(map[string]struct{})
	appendFlags := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if _, duplicate := seenFlags[f.Name]; duplicate {
				return
			}
			seenFlags[f.Name] = struct{}{}
			h.Flags = append(h.Flags, jsonFlag{
				Name:        "--" + f.Name,
				Shorthand:   f.Shorthand,
				Description: f.Usage,
				Type:        f.Value.Type(),
				Default:     f.DefValue,
			})
		})
	}

	// NonInheritedFlags contains flags owned by this command; inherited flags
	// are added separately. cmd.Flags() merges both sets after parsing, which
	// caused global flags to appear twice in JSON help.
	appendFlags(cmd.NonInheritedFlags())
	appendFlags(cmd.InheritedFlags())

	printJSON(h)
}

// printDefaultHelp renders Cobra's built-in plain-text help by temporarily
// clearing the custom help function on the command and all its ancestors.
// Clearing only `cmd` is not enough: cobra's Help() walks up the parent chain
// to resolve a help func, which would re-enter wrapHelp on the still-wrapped
// root and recurse forever in non-TTY mode.
func printDefaultHelp(cmd *cobra.Command) {
	type saved struct {
		cmd *cobra.Command
		fn  func(*cobra.Command, []string)
	}
	var stack []saved
	for c := cmd; c != nil; c = c.Parent() {
		stack = append(stack, saved{c, c.HelpFunc()})
		c.SetHelpFunc(nil)
	}
	cmd.Help() //nolint:errcheck // stdout write failure is non-recoverable
	for _, s := range stack {
		s.cmd.SetHelpFunc(s.fn)
	}
}
