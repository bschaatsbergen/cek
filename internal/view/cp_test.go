package view_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCpHumanView(t *testing.T) {
	tests := []struct {
		name string
		data view.CpData
		want string
	}{
		{
			name: "single file",
			data: view.CpData{ImageRef: "alpine:latest", Source: "/etc/os-release", Destination: "os-release", Files: 1, Bytes: 188},
			want: "Copied 1 file (188 B) from alpine:latest:/etc/os-release to os-release\n",
		},
		{
			name: "directory",
			data: view.CpData{ImageRef: "alpine:latest", Source: "/etc/apk", Destination: "out/apk", Files: 5, Bytes: 1400},
			want: "Copied 5 files (1.4 KB) from alpine:latest:/etc/apk to out/apk\n",
		},
		{
			name: "skipped special files",
			data: view.CpData{ImageRef: "alpine:latest", Source: "/dev", Destination: "out/dev", Files: 0, Bytes: 0, Skipped: 3},
			want: "Copied 0 files (0 B) from alpine:latest:/dev to out/dev, skipped 3 special files\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)
			require.NoError(t, hv.Cp().Render(&tt.data))
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func TestCpJSONView(t *testing.T) {
	buf := new(bytes.Buffer)
	jv := view.NewJSONView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, jv.Cp().Render(&view.CpData{
		ImageRef: "alpine:latest", Source: "/etc/apk", Destination: "out/apk", Files: 5, Bytes: 1400, Skipped: 1,
	}))

	var output struct {
		Image       string `json:"image"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
		Files       int    `json:"files"`
		Bytes       int64  `json:"bytes"`
		Skipped     int    `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	assert.Equal(t, "alpine:latest", output.Image)
	assert.Equal(t, "/etc/apk", output.Source)
	assert.Equal(t, "out/apk", output.Destination)
	assert.Equal(t, 5, output.Files)
	assert.Equal(t, int64(1400), output.Bytes)
	assert.Equal(t, 1, output.Skipped)
}
