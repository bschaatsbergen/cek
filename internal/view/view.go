package view

var _ Viewer = (*HumanView)(nil)
var _ Viewer = (*JSONView)(nil)

// Viewer represents an output formatting strategy. Each view type (e.g., Human,
// JSON) implements this interface to support different views (e.g.,
// InspectView, CatView, LsView).
type Viewer interface {
	Inspect() InspectView
	Cat() CatView
	Ls() LsView
	Export() ExportView
	Tags() TagsView
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

func (h *HumanView) Cat() CatView {
	return newCatHumanView(h)
}

func (h *HumanView) Ls() LsView {
	return newLsHumanView(h)
}

func (h *HumanView) Export() ExportView {
	return newExportHumanView(h)
}

func (h *HumanView) Tags() TagsView {
	return newTagsHumanView(h)
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

func (j *JSONView) Cat() CatView {
	return newCatJSONView(j)
}

func (j *JSONView) Ls() LsView {
	return newLsJSONView(j)
}

func (j *JSONView) Export() ExportView {
	return newExportJSONView(j)
}

func (j *JSONView) Tags() TagsView {
	return newTagsJSONView(j)
}

func (j *JSONView) Logger() Logger {
	return j.logger
}
