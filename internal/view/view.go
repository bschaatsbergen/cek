package view

var _ Viewer = (*HumanView)(nil)
var _ Viewer = (*JSONView)(nil)

// Viewer represents an output formatting strategy.
// Each view type (e.g., Human, JSON) implements this interface to support different views (e.g., FmtView, PlanView).
type Viewer interface {
	Inspect() InspectView
	Logger() Logger
}

func NewViewer(vt ViewType, s *Stream, level LogLevel) Viewer {
	switch vt {
	case ViewHuman:
		return NewHumanView(s, level)
	case ViewJSON:
		return NewJSONView(s, level)
	default:
		panic("unknown view type")
	}
}

type HumanView struct {
	*Stream
	logger Logger
}

func NewHumanView(s *Stream, level LogLevel) *HumanView {
	var logger Logger
	if level == LogLevelSilent {
		logger = NewNopLogger()
	} else {
		logger = NewHumanLogger(s.Writer, level)
	}
	return &HumanView{
		Stream: s,
		logger: logger,
	}
}

func (h *HumanView) Inspect() InspectView {
	return newInspectHumanView(h)
}

func (h *HumanView) Logger() Logger {
	return h.logger
}

type JSONView struct {
	*Stream
	logger Logger
}

func NewJSONView(s *Stream, level LogLevel) *JSONView {
	var logger Logger
	if level == LogLevelSilent {
		logger = NewNopLogger()
	} else {
		logger = NewJSONLogger(s.Writer, level)
	}
	return &JSONView{
		Stream: s,
		logger: logger,
	}
}

func (j *JSONView) Inspect() InspectView {
	return newInspectJSONView(j)
}

func (j *JSONView) Logger() Logger {
	return j.logger
}
