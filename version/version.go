package version

import (
	_ "embed"
	"fmt"
	"io"
	"runtime"
)

// The version number that is being run at the moment, set through ldflags.
var Version string = "dev"

func Fprint(w io.Writer) {
	_, _ = fmt.Fprintf(w, "cek version %s\n", Version)
	_, _ = fmt.Fprintf(w, "%s/%s\n", runtime.GOOS, runtime.GOARCH)
}
