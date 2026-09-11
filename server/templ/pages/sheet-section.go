package pages
import "github.com/a-h/templ"
type shellLayout struct {
	Fill bool
	Back backTarget
	Actions templ.Component
	SubNav templ.Component
	OwnShare bool
}
func panelFormID(panel string) string {
	return "panel-" + panel
}

