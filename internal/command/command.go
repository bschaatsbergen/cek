package command

import (
	"fmt"
	"io"

	"github.com/bschaatsbergen/cek/internal/spinner"
	"github.com/bschaatsbergen/cek/internal/view"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// CLI is a global context passed to all commands.
// Unlike a Command which is specific to a single operation,
// CLI holds shared state and is propagated from root to subcommands.
type CLI struct {
	view.Viewer
	*view.Stream
	Endpoint        string
	Context         string
	ContextFlag     string
	DisableProgress bool
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

// ExactArgs returns an error if there is not the exact number of args.
func ExactArgs(number int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == number {
			return nil
		}
		return fmt.Errorf("expected %d arguments, got %d", number, len(args))
	}
}

// ExactArgsWithUsage returns an error if there is not the exact number of args,
// and shows usage information for better user experience.
func ExactArgsWithUsage(number int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == number {
			return nil
		}
		_ = cmd.Usage()
		if number == 1 {
			return fmt.Errorf("requires exactly 1 argument")
		}
		return fmt.Errorf("requires exactly %d arguments", number)
	}
}

// MaxArgsWithUsage returns an error if there are more than the maximum number of args,
// and shows usage information for better user experience.
func MaxArgsWithUsage(maxArgs int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) <= maxArgs {
			return nil
		}
		_ = cmd.Usage()
		if maxArgs == 1 {
			return fmt.Errorf("accepts at most 1 argument")
		}
		return fmt.Errorf("accepts at most %d arguments", maxArgs)
	}
}

// MaxArgs returns an error if there are more than the max number of args.
func MaxArgs(number int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) <= number {
			return nil
		}
		return fmt.Errorf("expected at most %d arguments, got %d", number, len(args))
	}
}

// RunWithSpinner runs a function that returns a generic result while showing a spinner (when enabled).
// It guarantees the spinner is stopped and stderr cleaned before returning so the caller can safely render to stdout.
func RunWithSpinner[T any](cli *CLI, message string, fn func() (*T, error)) (*T, error) {
	if !cli.DisableProgress {
		s := spinner.New(message)
		s.Start()
		defer s.Stop()
	}
	return fn()
}
