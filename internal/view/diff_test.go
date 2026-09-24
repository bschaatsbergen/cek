package view_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diffFixture() *view.DiffData {
	return &view.DiffData{
		A: "app:v1",
		B: "app:v2",
		Layers: []view.LayerDiff{
			{Status: view.DiffShared, Digest: "sha256:aaaa", Size: 1024},
			{Status: view.DiffRemoved, Digest: "sha256:bbbb", Size: 2048},
			{Status: view.DiffAdded, Digest: "sha256:cccc", Size: 4096},
		},
		Files: []view.FileDiff{
			{
				Status: view.DiffModified,
				Path:   "/app/config.yaml",
				A:      &view.FileState{Mode: "-rw-r--r--", Size: 120, Digest: "sha256:1111"},
				B:      &view.FileState{Mode: "-rw-r--r--", Size: 130, Digest: "sha256:2222"},
			},
			{
				Status: view.DiffModified,
				Path:   "/app/run.sh",
				A:      &view.FileState{Mode: "-rw-r--r--", Size: 10, Digest: "sha256:3333"},
				B:      &view.FileState{Mode: "-rwxr-xr-x", Size: 10, Digest: "sha256:3333"},
			},
			{
				Status: view.DiffModified,
				Path:   "/app/current",
				A:      &view.FileState{Mode: "lrwxrwxrwx", Link: "v1"},
				B:      &view.FileState{Mode: "lrwxrwxrwx", Link: "v2"},
			},
			{
				Status: view.DiffAdded,
				Path:   "/app/new.txt",
				B:      &view.FileState{Mode: "-rw-r--r--", Size: 12, Digest: "sha256:4444"},
			},
			{
				Status: view.DiffRemoved,
				Path:   "/app/old",
				A:      &view.FileState{Mode: "lrwxrwxrwx", Link: "/gone"},
			},
		},
	}
}

func TestDiffHumanView_RendersLayersAndFiles(t *testing.T) {
	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Diff().Render(diffFixture()))

	output := buf.String()
	assert.Contains(t, output, "Layers:\n  = sha256:aaaa  1.0 KB\n  - sha256:bbbb  2.0 KB\n  + sha256:cccc  4.0 KB\n")
	assert.Contains(t, output, "Files:\n")
	assert.Regexp(t, `~ /app/config.yaml\s+120 B -> 130 B\n`, output)
	assert.Regexp(t, `~ /app/run.sh\s+mode -rw-r--r-- -> -rwxr-xr-x\n`, output)
	assert.Regexp(t, `~ /app/current\s+link v1 -> v2\n`, output)
	assert.Regexp(t, `\+ /app/new.txt\s+12 B\n`, output)
	assert.Regexp(t, `- /app/old\s+-> /gone\n`, output)
	assert.Contains(t, output, "\n1 added, 1 removed, 3 modified\n")
}

func TestDiffHumanView_NoChanges(t *testing.T) {
	data := diffFixture()
	data.Files = nil

	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Diff().Render(data))

	output := buf.String()
	assert.Contains(t, output, "Files: no changes\n")
	assert.NotContains(t, output, "added,")
}

func TestDiffJSONView(t *testing.T) {
	buf := new(bytes.Buffer)
	jv := view.NewJSONView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, jv.Diff().Render(diffFixture()))

	var output struct {
		A      string `json:"a"`
		B      string `json:"b"`
		Layers []struct {
			Status string `json:"status"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"layers"`
		Files []struct {
			Status string `json:"status"`
			Path   string `json:"path"`
			A      *struct {
				Mode   string `json:"mode"`
				Size   int64  `json:"size"`
				Link   string `json:"link"`
				Digest string `json:"digest"`
			} `json:"a"`
			B *struct {
				Mode   string `json:"mode"`
				Size   int64  `json:"size"`
				Digest string `json:"digest"`
			} `json:"b"`
		} `json:"files"`
		Summary struct {
			Added    int `json:"added"`
			Removed  int `json:"removed"`
			Modified int `json:"modified"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))

	assert.Equal(t, "app:v1", output.A)
	assert.Equal(t, "app:v2", output.B)
	require.Len(t, output.Layers, 3)
	assert.Equal(t, "shared", output.Layers[0].Status)
	assert.Equal(t, "removed", output.Layers[1].Status)
	assert.Equal(t, "added", output.Layers[2].Status)

	require.Len(t, output.Files, 5)
	assert.Equal(t, "modified", output.Files[0].Status)
	assert.Equal(t, "sha256:1111", output.Files[0].A.Digest)
	assert.Equal(t, "sha256:2222", output.Files[0].B.Digest)
	assert.Nil(t, output.Files[3].A, "an added file has no a side")
	assert.Nil(t, output.Files[4].B, "a removed file has no b side")
	assert.Equal(t, "/gone", output.Files[4].A.Link)

	assert.Equal(t, 1, output.Summary.Added)
	assert.Equal(t, 1, output.Summary.Removed)
	assert.Equal(t, 3, output.Summary.Modified)
}
