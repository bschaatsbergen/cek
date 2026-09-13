package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/bschaatsbergen/cek/version"
)

var rootCmd *cobra.Command

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "cek",
		Short: color.RGB(50, 108, 229).Sprintf("cek [global options] <subcommand> [args]") + `\n` +
			"List, inspect and explore OCI images and their layers",
		Long: color.RGB(50, 108, 229).Sprintf("Usage: cek [global options] <subcommand> [args]\n") +
			`
_________ _______________  __.
\_   ___ \\_   _____/    |/ _|
/    \  \/ |    __)_|      <  
\     \____|        \    |  \ 
 \______  /_______  /____|__ \
        \/        \/        \/
		` + "\n" +
			"List, inspect and explore OCI images and their layers.\n\n" +
			"cek provides commands to interact with OCI container images,\n" +
			"allowing you to inspect manifests, explore layers, examine files,\n" +
			"and compare images.\n",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				_ = cmd.Help()
			}
		},
	}

	cmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	cmd.PersistentFlags().Bool("debug", false, "Set log level to debug")
	return cmd
}

// ConfigureView replaces the CLI's view once cobra has parsed the global
// flags. Those flags are only known after the subcommand is resolved, so
// the view cannot be chosen before Execute. PersistentPreRun runs after
// parsing and before any RunE reads the view, for every subcommand.
func ConfigureView(root *cobra.Command, cli *CLI) {
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		debug, _ := cmd.Flags().GetBool("debug")

		viewType := view.ViewHuman
		if jsonOutput {
			viewType = view.ViewJSON
		}

		cli.Viewer = view.NewViewer(viewType, cli.Stream, logLevel(debug))
	}
}

// logLevel derives the log level from the --debug flag and the CEK_LOG
// environment variable. The flag wins.
func logLevel(debug bool) view.LogLevel {
	if debug {
		return view.LogLevelDebug
	}
	switch strings.ToLower(os.Getenv("CEK_LOG")) {
	case "debug":
		return view.LogLevelDebug
	case "info":
		return view.LogLevelInfo
	default:
		return view.LogLevelSilent
	}
}

func setCobraUsageTemplate() {
	cobra.AddTemplateFunc("StyleHeading", color.RGB(50, 108, 229).SprintFunc())
	usageTemplate := rootCmd.UsageTemplate()
	usageTemplate = strings.NewReplacer(
		`Usage:`, `{{StyleHeading "Usage:"}}`,
		`Examples:`, `{{StyleHeading "Examples:"}}`,
		`Available Commands:`, `{{StyleHeading "Available Commands:"}}`,
		`Additional Commands:`, `{{StyleHeading "Additional Commands:"}}`,
		`Flags:`, `{{StyleHeading "Options:"}}`,
		`Global Flags:`, `{{StyleHeading "Global Options:"}}`,
	).Replace(usageTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)
}

func setVersionTemplate() {
	rootCmd.SetVersionTemplate("{{.Version}}")
}

func Execute() {
	rootCmd = NewRootCommand()

	// Templates are used to standardize the output format of the CLI.
	setCobraUsageTemplate()
	setVersionTemplate()

	// Disable color output if NO_COLOR is set in the environment
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		color.NoColor = true
	} else {
		color.NoColor = false
	}

	// Create a new CLI instance, which is a global context that each command
	// can use to access, useful for view rendering, etc. It starts with the
	// human view; ConfigureView swaps it once the global flags are parsed.
	cli := NewCLI(view.ViewHuman, os.Stdout, view.LogLevelSilent)
	ConfigureView(rootCmd, cli)

	// Add all subcommands to the root command
	AddCommands(rootCmd, cli)

	// Walk and execute the resolved command with flags.
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// AddCommands registers all subcommands to the root command.
func AddCommands(root *cobra.Command, cli *CLI) {
	root.AddCommand(
		NewVersionCommand(cli),
		NewInspectCommand(cli),
		NewCatCommand(cli),
		NewLsCommand(cli),
		NewTagsCommand(cli),
		NewExportCommand(cli),
		NewTreeCommand(cli),
		NewBlobCommand(cli),
	)
}
