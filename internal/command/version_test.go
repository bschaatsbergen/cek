package command

import (
	"bytes"
	"testing"

	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := newVersionCommand(cli)

	assert.Equal(t, "version", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestVersionCommand_Execute(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := newVersionCommand(cli)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestVersionCommand_WithPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := newVersionCommand(cli)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestVersionCommand_TooManyArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := newVersionCommand(cli)
	cmd.SetArgs([]string{"arg1", "arg2"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected at most 1")
}
