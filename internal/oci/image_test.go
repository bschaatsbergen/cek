package oci_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

// newAuthRegistry starts a registry that only answers requests carrying the
// given basic auth credentials, publishes them to the docker keychain via a
// temporary DOCKER_CONFIG, and pushes a one-layer image to <host>/test:latest.
func newAuthRegistry(t *testing.T, user, pass string) (host string) {
	t.Helper()

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	backend := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	host = strings.TrimPrefix(srv.URL, "http://")

	dir := t.TempDir()
	cfg := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, host, base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600))
	t.Setenv("DOCKER_CONFIG", dir)

	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	ref, err := name.ParseReference(host + "/test:latest")
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img, remote.WithAuth(&authn.Basic{Username: user, Password: pass})))

	return host
}

func TestFetchImage_UsesDockerCredentials(t *testing.T) {
	host := newAuthRegistry(t, "user", "secret")

	img, _, err := oci.FetchImage(t.Context(), host+"/test:latest", &oci.FetchOptions{PullPolicy: oci.PullAlways})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

func TestFetchImage_FailsWithoutCredentials(t *testing.T) {
	host := newAuthRegistry(t, "user", "secret")
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	_, _, err := oci.FetchImage(t.Context(), host+"/test:latest", &oci.FetchOptions{PullPolicy: oci.PullAlways})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestListTags_UsesDockerCredentials(t *testing.T) {
	host := newAuthRegistry(t, "user", "secret")

	repo, err := name.NewRepository(host + "/test")
	require.NoError(t, err)

	tags, err := oci.ListTags(t.Context(), repo)
	require.NoError(t, err)
	assert.Equal(t, []string{"latest"}, tags)
}
