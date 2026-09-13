package oci_test

import (
	"testing"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePullPolicy(t *testing.T) {
	tests := []struct {
		input   string
		want    oci.PullPolicy
		wantErr bool
	}{
		{"always", oci.PullAlways, false},
		{"if-not-present", oci.PullIfNotPresent, false},
		{"never", oci.PullNever, false},
		{"", "", true},
		{"Always", "", true},
		{"bogus", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := oci.ParsePullPolicy(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid pull policy")
				assert.Contains(t, err.Error(), "always, if-not-present, never")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchImage_RejectsInvalidPullPolicy(t *testing.T) {
	_, _, err := oci.FetchImage(t.Context(), "alpine:latest", &oci.FetchOptions{PullPolicy: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid pull policy "bogus"`)
}
