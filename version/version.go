package version

import (
	_ "embed"
	"fmt"
	"io"
	"runtime"
	"strings"
)

//go:embed VERSION
var versionFile string

var (
	Version string
)

func init() {
	if Version == "" {
		Version = strings.TrimSpace(versionFile)
	}

	if Version == "" {
		Version = "dev"
	}
}

func Print() {
	fmt.Printf("cek version %s\n", Version)
	fmt.Printf("%s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func Fprint(w io.Writer) {
	_, _ = fmt.Fprintf(w, "cek version %s\n", Version)
	_, _ = fmt.Fprintf(w, "%s/%s\n", runtime.GOOS, runtime.GOARCH)
}
