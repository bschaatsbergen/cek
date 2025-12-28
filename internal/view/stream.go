package view

import (
	"fmt"
	"io"

	"github.com/bschaatsbergen/cek/version"
)

type Stream struct {
	Writer io.Writer
}

func NewStream(w io.Writer) *Stream {
	return &Stream{
		Writer: w,
	}
}

func (s *Stream) Println(args ...any) {
	_, _ = fmt.Fprintln(s.Writer, args...)
}

func (s *Stream) Printf(fmtStr string, args ...any) {
	_, _ = fmt.Fprintf(s.Writer, fmtStr, args...)
}

func (s *Stream) PrintVersion() {
	version.Fprint(s.Writer)
}
