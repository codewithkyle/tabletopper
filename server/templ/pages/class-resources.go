package pages

import "strconv"

const (
	ResourceIDField   = "resources-id"
	ResourceNameLimit = 64
	ResourceMaxLimit  = 999
	ResourcePipLimit  = 10
	ResourceRowLimit  = 20
)

type ClassResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Max  int    `json:"max"`
	Used int    `json:"used"`
}

func ResourceField(id string, part string) string {
	return "resource-" + id + "-" + part
}
func (r ClassResource) Pips() bool {
	return r.Max <= ResourcePipLimit
}
func (r ClassResource) Left() string {
	return strconv.Itoa(r.Max-r.Used) + " of " + strconv.Itoa(r.Max) + " left"
}
func (r ClassResource) Label() string {
	if r.Name == "" {
		return "Resource"
	}
	return r.Name
}
func resourceCounterID(id string) string {
	return "resource-counter-" + id
}
func resourceCounterURL(id string) string {
	return "/fragment/character/resource-counter?id=" + id
}

const resourceNameList = "class-resource-names"

func resourceNames() []string {
	return []string{
		"Bardic Inspiration",
		"Channel Divinity",
		"Focus Points",
		"Psionic Energy Dice",
		"Rage",
		"Second Wind",
		"Sorcery Points",
		"Superiority Dice",
		"Wild Shape",
	}
}
