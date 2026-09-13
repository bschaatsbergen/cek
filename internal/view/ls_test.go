package view_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lsFixture() *view.LsData {
	return &view.LsData{
		Files: []view.FileInfo{
			{Mode: "drwxr-xr-x", Size: 0, Path: "/etc"},
			{Mode: "-rw-r--r--", Size: 12, Path: "/etc/hostname"},
		},
	}
}

func TestLsHumanView_DirectoriesEndInSlash(t *testing.T) {
	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Ls().Render(lsFixture()))

	output := buf.String()
	assert.Contains(t, output, "  /etc/\n")
	assert.Contains(t, output, "  /etc/hostname\n")
}

func TestLsJSONView_PathsAreClean(t *testing.T) {
	buf := new(bytes.Buffer)
	jv := view.NewJSONView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, jv.Ls().Render(lsFixture()))

	var output struct {
		Files []struct {
			Mode string `json:"mode"`
			Path string `json:"path"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	require.Len(t, output.Files, 2)
	assert.Equal(t, "/etc", output.Files[0].Path)
	assert.Equal(t, "drwxr-xr-x", output.Files[0].Mode)
}
