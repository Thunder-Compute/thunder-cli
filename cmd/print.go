package cmd

import (
	"fmt"
	"os"

	"github.com/Thunder-Compute/thunder-cli/tui"
)

func PrintError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, tui.RenderError(err))
	}
}

func FormatError(err error) string {
	return tui.RenderError(err)
}

func PrintWarning(message string) {
	if message != "" {
		fmt.Println(tui.RenderWarning(message))
	}
}

func PrintWarningSimple(message string) {
	if message != "" {
		fmt.Println(tui.RenderWarningSimple(message))
	}
}

func FormatWarning(message string) string {
	return tui.RenderWarning(message)
}

func FormatWarningSimple(message string) string {
	return tui.RenderWarningSimple(message)
}

func PrintSuccess(message string) {
	if message != "" {
		fmt.Println(tui.RenderSuccess(message))
	}
}

func PrintSuccessSimple(message string) {
	if message != "" {
		fmt.Println(tui.RenderSuccessSimple(message))
	}
}

func FormatSuccess(message string) string {
	return tui.RenderSuccess(message)
}

func FormatSuccessSimple(message string) string {
	return tui.RenderSuccessSimple(message)
}
