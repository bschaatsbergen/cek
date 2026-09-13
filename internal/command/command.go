package command

import (
	"io"

	"github.com/bschaatsbergen/cek/internal/view"

	"github.com/fatih/color"
)

// CLI is a global context passed to all commands.
// Unlike a Command which is specific to a single operation,
// CLI holds shared state and is propagated from root to subcommands.
type CLI struct {
	view.Viewer
	*view.Stream
}

// highlight applies a blue color to the given format and arguments.
func highlight(format string, a ...any) string {
	return color.RGB(50, 108, 229).Sprintf(format, a...)
}

func NewCLI(vt view.ViewType, w io.Writer, logLevel view.LogLevel) *CLI {
	s := view.NewStream(w)

	return &CLI{
		Viewer: view.NewViewer(vt, s, logLevel),
		Stream: s,
	}
}
