package pages

import (
	"regexp"
	"strings"
	"testing"
)

func TestVitalsCarryTheResourceTrackerInsideTheirOwnForm(t *testing.T) {
	sheet := testSheetPage()
	body := renderString(t, vitalsPanel(sheet))
	if got := strings.Count(body, "<form"); got != 1 {
		t.Fatalf("the vitals panel renders %d forms, want the one that saves it", got)
	}
	rage, sorcery := sheet.Resources[0], sheet.Resources[1]
	for _, want := range []string{
		">Class Resources<",
		`<datalist id="` + resourceNameList + `">`,
		`<option value="Sorcery Points">`,
		`hx-get="/fragment/character/resource-row"`,
		`list="` + resourceNameList + `"`,
		`hx-get="` + resourceCounterURL(rage.ID) + `"`,
		`hx-target="#` + resourceCounterID(rage.ID) + `"`,
		`hx-include="closest [data-resource-row]"`,
		`name="` + ResourceField(sorcery.ID, "used") + `"`,
		">16 left<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the vitals panel is missing %s", want)
		}
	}
	if got := strings.Count(body, `name="`+ResourceField(rage.ID, "spent")+`"`); got != rage.Max {
		t.Errorf("rage draws %d boxes, want %d", got, rage.Max)
	}
	if strings.Contains(renderString(t, classResources(sheet.Resources)), "placeholder=") {
		t.Error("the tracker carries placeholder text")
	}
}
func TestDeletingAResourceAsksFirst(t *testing.T) {
	body := renderString(t, resourceRow(ClassResource{ID: "01JA0000000000000000000001", Name: "Rage", Max: 3}))
	button := regexp.MustCompile(`<button[^>]*aria-label="Delete resource"[^>]*>`).FindString(body)
	if button == "" {
		t.Fatalf("no delete button:\n%s", body)
	}
	for _, want := range []string{`hx-confirm="`, `data-remove-closest="[data-resource-row]"`} {
		if !strings.Contains(button, want) {
			t.Errorf("the delete button is missing %s: %s", want, button)
		}
	}
}
func TestAResourceWithNoMaximumDrawsNoCounter(t *testing.T) {
	body := renderString(t, ResourceCounter(ClassResource{ID: "01JA0000000000000000000001", Name: "Rage"}))
	if strings.Contains(body, "<input") || strings.Contains(body, "left") {
		t.Errorf("an unset maximum still draws a counter:\n%s", body)
	}
}
