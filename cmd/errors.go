package cmd

import (
	"fmt"
	"os"
)

const (
	ansiRed       = "\033[31m"
	ansiYellow    = "\033[33m"
	ansiGreen     = "\033[32m"
	ansiBold      = "\033[1m"
	ansiReset     = "\033[0m"
	errorPrefix   = "✗ Error: "
	warningPrefix = "⚠ Warning: "
	successPrefix = "✓ Success: "
)

func PrintError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%s%s%s%s\n", ansiBold, ansiRed, errorPrefix, err.Error(), ansiReset)
	}
}

func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s%s%s%s%s", ansiBold, ansiRed, errorPrefix, err.Error(), ansiReset)
}

func PrintWarning(message string) {
	if message != "" {
		fmt.Printf("%s%s%s%s%s\n", ansiBold, ansiYellow, warningPrefix, message, ansiReset)
	}
}

func PrintWarningSimple(message string) {
	if message != "" {
		fmt.Printf("%s%s%s%s%s\n", ansiBold, ansiYellow, "⚠ ", message, ansiReset)
	}
}

func FormatWarning(message string) string {
	if message == "" {
		return ""
	}
	return fmt.Sprintf("%s%s%s%s%s", ansiBold, ansiYellow, warningPrefix, message, ansiReset)
}

func FormatWarningSimple(message string) string {
	if message == "" {
		return ""
	}
	return fmt.Sprintf("%s%s%s%s%s", ansiBold, ansiYellow, "⚠ ", message, ansiReset)
}

func PrintSuccess(message string) {
	if message != "" {
		fmt.Printf("%s%s%s%s%s\n", ansiBold, ansiGreen, successPrefix, message, ansiReset)
	}
}

func PrintSuccessSimple(message string) {
	if message != "" {
		fmt.Printf("%s%s%s%s%s\n", ansiBold, ansiGreen, "✓ ", message, ansiReset)
	}
}

func FormatSuccess(message string) string {
	if message == "" {
		return ""
	}
	return fmt.Sprintf("%s%s%s%s%s", ansiBold, ansiGreen, successPrefix, message, ansiReset)
}

func FormatSuccessSimple(message string) string {
	if message == "" {
		return ""
	}
	return fmt.Sprintf("%s%s%s%s%s", ansiBold, ansiGreen, "✓ ", message, ansiReset)
}
