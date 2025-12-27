package view

import (
	"github.com/fatih/color"
)

type InspectView interface {
	Render()
}

// Human view implementation
type inspectHumanView struct {
	*HumanView
}

func newInspectHumanView(hv *HumanView) *inspectHumanView {
	return &inspectHumanView{HumanView: hv}
}

func (v *inspectHumanView) Render() {
	v.Println(color.RGB(50, 108, 229).Sprintf("Success!"), "The infrastructure was inspected successfully")
}

// JSON view implementation
type inspectJSONView struct {
	*JSONView
}

func newInspectJSONView(jv *JSONView) *inspectJSONView {
	return &inspectJSONView{JSONView: jv}
}
func (v *inspectJSONView) Render() {
}
