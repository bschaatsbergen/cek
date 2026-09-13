package view_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bschaatsbergen/cek/internal/view"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inspectFixture() *view.InspectData {
	return &view.InspectData{
		ImageRef:     "localhost:5001/demo/app:1.0-enc",
		Registry:     "localhost:5001",
		Digest:       v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)},
		Created:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		OS:           "linux",
		Architecture: "arm64",
		TotalSize:    3072,
		Layers: []view.LayerData{
			{
				Index:     1,
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)},
				Size:      1024,
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			},
			{
				Index:     2,
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("c", 64)},
				Size:      2048,
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip+encrypted",
				Annotations: map[string]string{
					"org.opencontainers.image.enc.pubopts":  "eyJjaXBoZXIiOiJBRVNfMjU2X0NUUl9ITUFDX1NIQTI1NiJ9",
					"org.opencontainers.image.enc.keys.jwe": "eyJwcm90ZWN0ZWQiOiJleUoifQ",
				},
			},
		},
	}
}

func TestInspectHumanView_RendersMediaType(t *testing.T) {
	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Inspect().Render(inspectFixture()))

	output := buf.String()
	assert.Contains(t, output, "Media Type")
	assert.Contains(t, output, "application/vnd.oci.image.layer.v1.tar+gzip\n")
	assert.Contains(t, output, "application/vnd.oci.image.layer.v1.tar+gzip+encrypted")
}

func TestInspectHumanView_RendersLayerAnnotations(t *testing.T) {
	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Inspect().Render(inspectFixture()))

	output := buf.String()
	assert.Contains(t, output, "Layer annotations:")

	jwe := strings.Index(output, "org.opencontainers.image.enc.keys.jwe")
	pubopts := strings.Index(output, "org.opencontainers.image.enc.pubopts")
	require.NotEqual(t, -1, jwe)
	require.NotEqual(t, -1, pubopts)
	assert.Less(t, jwe, pubopts, "annotation keys should be sorted")

	assert.Contains(t, output, "eyJwcm90ZWN0ZWQiOiJleUoifQ")
	assert.Contains(t, output, "eyJjaXBoZXIiOiJBRVNfMjU2X0NUUl9ITUFDX1NIQTI1NiJ9")

	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "org.opencontainers.image.enc.") {
			assert.True(t, strings.HasPrefix(line, "2"), "annotation row should carry its layer index: %q", line)
		}
	}
}

func TestInspectHumanView_NoAnnotations_OmitsSection(t *testing.T) {
	data := inspectFixture()
	for i := range data.Layers {
		data.Layers[i].Annotations = nil
	}

	buf := new(bytes.Buffer)
	hv := view.NewHumanView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, hv.Inspect().Render(data))
	assert.NotContains(t, buf.String(), "Layer annotations:")
}

func TestInspectJSONView_RendersMediaTypeAndAnnotations(t *testing.T) {
	buf := new(bytes.Buffer)
	jv := view.NewJSONView(view.NewStream(buf), view.LogLevelSilent)

	require.NoError(t, jv.Inspect().Render(inspectFixture()))

	var output struct {
		Layers []struct {
			Index       int               `json:"index"`
			Digest      string            `json:"digest"`
			Size        int64             `json:"size"`
			MediaType   string            `json:"mediaType"`
			Annotations map[string]string `json:"annotations"`
		} `json:"layers"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	require.Len(t, output.Layers, 2)

	assert.Equal(t, "application/vnd.oci.image.layer.v1.tar+gzip", output.Layers[0].MediaType)
	assert.Nil(t, output.Layers[0].Annotations)
	assert.NotContains(t, buf.String(), `"annotations": null`)

	assert.Equal(t, "application/vnd.oci.image.layer.v1.tar+gzip+encrypted", output.Layers[1].MediaType)
	assert.Equal(t, map[string]string{
		"org.opencontainers.image.enc.keys.jwe": "eyJwcm90ZWN0ZWQiOiJleUoifQ",
		"org.opencontainers.image.enc.pubopts":  "eyJjaXBoZXIiOiJBRVNfMjU2X0NUUl9ITUFDX1NIQTI1NiJ9",
	}, output.Layers[1].Annotations)
}
