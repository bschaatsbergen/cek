package command_test

import (
	"bytes"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewVersionCommand(cli)

	assert.Equal(t, "version", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestVersionCommand_Execute(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewVersionCommand(cli)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestVersionCommand_WithPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewVersionCommand(cli)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestVersionCommand_TooManyArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewVersionCommand(cli)
	cmd.SetArgs([]string{"arg1", "arg2"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected at most 1")
}
