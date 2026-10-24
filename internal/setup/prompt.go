package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// UI is how setup talks to the person running it.
type UI struct {
	In  *bufio.Reader
	Out io.Writer
	Yes bool // --yes: accept every default
}

func NewUI(yes bool) *UI {
	return &UI{In: bufio.NewReader(os.Stdin), Out: os.Stderr, Yes: yes}
}

func (u *UI) Say(format string, a ...any) { fmt.Fprintf(u.Out, format+"\n", a...) }

func (u *UI) Step(n int, title string) { fmt.Fprintf(u.Out, "\n[%d] %s\n", n, title) }

// Confirm asks a yes/no question. With --yes, or when nothing can be read,
// it takes the default.
func (u *UI) Confirm(q string, def bool) bool {
	if u.Yes {
		return def
	}
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(u.Out, "%s %s ", q, hint)
	line, err := u.In.ReadString('\n')
	if err != nil {
		fmt.Fprintln(u.Out)
		return def
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}

// Run prints a command, then runs it with output shown, so nothing happens
// to the host that the person didn't see.
func (u *UI) Run(name string, args ...string) error {
	fmt.Fprintf(u.Out, "  $ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = u.Out, u.Out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// Shell is Run for a pipeline.
func (u *UI) Shell(script string) error { return u.Run("sh", "-c", script) }
