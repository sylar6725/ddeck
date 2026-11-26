// ddeck — a TUI for dd: disk cloning with tracking of several copies at once.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"ddeck/internal/ui"
)

func main() {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "ddeck only runs on Linux.")
		os.Exit(1)
	}
	for _, bin := range []string{"lsblk", "dd"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "command not found: %s\n", bin)
			os.Exit(1)
		}
	}

	m := ui.New(os.Geteuid() == 0)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetSender(p.Send)
	_, err := p.Run()
	m.Shutdown()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
