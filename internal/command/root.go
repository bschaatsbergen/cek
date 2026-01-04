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

var (
	jsonFlag  bool
	debugFlag bool
	rootCmd   *cobra.Command
)

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

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Set log level to debug")
	return cmd
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

	// Parse flags early so the root command is aware of global flags
	// before any subcommand executes. This is necessary to configure
	// things like the output format (view type) and writer upfront.
	_ = rootCmd.ParseFlags(os.Args[1:])

	// Disable color output if NO_COLOR is set in the environment
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		color.NoColor = true
	} else {
		color.NoColor = false
	}

	// Set up the view type based on the `--json` flag
	viewType := view.ViewHuman
	if jsonFlag {
		viewType = view.ViewJSON
	}

	logLevel := view.LogLevelSilent
	logEnv := os.Getenv("CEK_LOG")
	switch strings.ToLower(logEnv) {
	case "debug":
		logLevel = view.LogLevelDebug
	case "info":
		logLevel = view.LogLevelInfo
	default:
		// Unknown value: keep default (silent)
	}
	if debugFlag {
		logLevel = view.LogLevelDebug
	}

	// Create a new CLI instance, which is a global context that each command
	// can use to access, useful for view rendering, etc.
	cli := NewCLI(viewType, os.Stdout, logLevel)

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
	)
}
