package tui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// ShutdownProgram requests a Bubble Tea program to quit and waits for it to exit
// before restoring cursor state. The done channel should be closed by the
// goroutine running p.Run().
func ShutdownProgram(p *tea.Program, done <-chan error, out io.Writer) {
	if p != nil {
		go p.Quit()
	}
	if done != nil {
		<-done
	}
	ResetLine(out)
	ShowCursor(out)
}
