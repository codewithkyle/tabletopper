package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

const (
	testRageID    = "01JA0000000000000000000001"
	testSorceryID = "01JA0000000000000000000002"
	testBlankID   = "01JA0000000000000000000003"
)

func resourceForm(rows ...map[string][]string) url.Values {
	form := url.Values{}
	for _, row := range rows {
		for key, values := range row {
			form[key] = append(form[key], values...)
		}
	}
	return form
}
func resourceRow(id, name, most string, extra map[string][]string) map[string][]string {
	row := map[string][]string{
		pages.ResourceIDField:           {id},
		pages.ResourceField(id, "name"): {name},
		pages.ResourceField(id, "max"):  {most},
	}
	for key, values := range extra {
		row[key] = values
	}
	return row
}
func savedResources(t *testing.T, call recordedCall) []pages.ClassResource {
	t.Helper()
	for _, arg := range call.args {
		raw, ok := arg.(json.RawMessage)
		if !ok {
			continue
		}
		var rows []pages.ClassResource
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("resources are not JSON: %v", err)
		}
		return rows
	}
	t.Fatalf("the vitals update carried no resources: %v", call.args)
	return nil
}
func TestVitalsSaveCarriesItsResourcesCleanedUp(t *testing.T) {
	app, db := newPanelApp(1)
	form := resourceForm(
		resourceRow(testRageID, "  Rage ", "3", map[string][]string{pages.ResourceField(testRageID, "spent"): {"1", "1"}}),
		resourceRow(testSorceryID, "Sorcery Points", "20", map[string][]string{pages.ResourceField(testSorceryID, "used"): {"25"}}),
		resourceRow(testBlankID, "", "", nil),
		resourceRow("not-a-ulid", "Wild Shape", "2", nil),
		map[string][]string{pages.ResourceIDField: {testRageID}},
	)
	rec := panelPost(t, db, app.SaveCharacterVitals, form, map[string]string{"id": testCharacterID.String()})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\n%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	want := []pages.ClassResource{
		{ID: testRageID, Name: "Rage", Max: 3, Used: 2},
		{ID: testSorceryID, Name: "Sorcery Points", Max: 20, Used: 20},
	}
	if got := savedResources(t, db.only(t)); !reflect.DeepEqual(got, want) {
		t.Errorf("saved %+v, want %+v", got, want)
	}
}
func TestVitalsWithNoResourcesSaveAnEmptyList(t *testing.T) {
	app, db := newPanelApp(1)
	panelPost(t, db, app.SaveCharacterVitals, url.Values{}, map[string]string{"id": testCharacterID.String()})
	if got := savedResources(t, db.only(t)); got == nil || len(got) != 0 {
		t.Errorf("saved %#v, want an empty list rather than null", got)
	}
}
func TestVitalsRefuseResourcesTheyCannotHold(t *testing.T) {
	long := resourceForm(resourceRow(testRageID, strings.Repeat("a", pages.ResourceNameLimit+1), "3", nil))
	many := url.Values{}
	for i := range pages.ResourceRowLimit + 1 {
		id := ulid.Make().String()
		many = resourceForm(many, resourceRow(id, "Row "+strconv.Itoa(i), "1", nil))
	}
	for name, form := range map[string]url.Values{"a long name": long, "too many rows": many} {
		app, db := newPanelApp(1)
		rec := panelPost(t, db, app.SaveCharacterVitals, form, map[string]string{"id": testCharacterID.String()})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, http.StatusUnprocessableEntity)
		}
		if len(db.calls) != 0 {
			t.Errorf("%s: ran %d statements, want 0", name, len(db.calls))
		}
	}
}
func counterRequest(t *testing.T, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	(&App{}).ResourceCounterFragment(rec, httptest.NewRequest(http.MethodGet, "/fragment/character/resource-counter?"+query.Encode(), nil))
	return rec
}
func TestResourceCounterRefusesAnIDItDidNotIssue(t *testing.T) {
	for _, id := range []string{"", "nope", strings.ToLower(testRageID), "01JA000000000000000000000!"} {
		rec := counterRequest(t, url.Values{"id": {id}})
		if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
			t.Errorf("id %q: status %d with %d bytes, want an empty 404", id, rec.Code, rec.Body.Len())
		}
	}
}
func TestResourceCounterDrawsBoxesUpToTheLimitAndANumberPastIt(t *testing.T) {
	boxes := counterRequest(t, url.Values{
		"id":                                     {testRageID},
		pages.ResourceField(testRageID, "max"):   {strconv.Itoa(pages.ResourcePipLimit)},
		pages.ResourceField(testRageID, "spent"): {"1", "1"},
	}).Body.String()
	if got := strings.Count(boxes, `type="checkbox"`); got != pages.ResourcePipLimit {
		t.Errorf("drew %d boxes, want %d", got, pages.ResourcePipLimit)
	}
	if got := strings.Count(boxes, " checked"); got != 2 {
		t.Errorf("ticked %d boxes, want 2", got)
	}
	number := counterRequest(t, url.Values{
		"id": {testSorceryID},
		pages.ResourceField(testSorceryID, "max"):  {strconv.Itoa(pages.ResourcePipLimit + 1)},
		pages.ResourceField(testSorceryID, "used"): {"99"},
	}).Body.String()
	if strings.Contains(number, `type="checkbox"`) {
		t.Errorf("past the limit it still drew boxes:\n%s", number)
	}
	for _, want := range []string{`name="` + pages.ResourceField(testSorceryID, "used") + `"`, `value="11"`, ">0 left<"} {
		if !strings.Contains(number, want) {
			t.Errorf("the number counter is missing %s:\n%s", want, number)
		}
	}
}
func TestEveryNewResourceRowGetsItsOwnID(t *testing.T) {
	ids := map[string]bool{}
	for range 3 {
		rec := httptest.NewRecorder()
		(&App{}).ResourceRowFragment(rec, httptest.NewRequest(http.MethodGet, "/fragment/character/resource-row", nil))
		body := rec.Body.String()
		at := strings.Index(body, `name="`+pages.ResourceIDField+`" value="`)
		if at < 0 {
			t.Fatalf("the row carries no id:\n%s", body)
		}
		id := body[at+len(`name="`+pages.ResourceIDField+`" value="`):][:26]
		if _, ok := resourceID(id); !ok {
			t.Fatalf("the row's id %q is not one the save accepts", id)
		}
		if !strings.Contains(body, `name="`+pages.ResourceField(id, "name")+`"`) {
			t.Errorf("the row's fields are not named for its id %s", id)
		}
		ids[id] = true
	}
	if len(ids) != 3 {
		t.Errorf("three rows shared ids: %v", ids)
	}
}
func TestStoredResourcesAreCheckedOnTheWayOut(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":"` + testRageID + `","name":"Rage","max":3,"used":7},
		{"id":"bogus","name":"Wild Shape","max":2,"used":0},
		{"id":"` + testSorceryID + `","name":"Sorcery Points","max":5000,"used":-2}
	]`)
	want := []pages.ClassResource{
		{ID: testRageID, Name: "Rage", Max: 3, Used: 3},
		{ID: testSorceryID, Name: "Sorcery Points", Max: pages.ResourceMaxLimit, Used: 0},
	}
	if got := parseResources(raw); !reflect.DeepEqual(got, want) {
		t.Errorf("parseResources = %+v, want %+v", got, want)
	}
	if got := parseResources(json.RawMessage(`not json`)); got != nil {
		t.Errorf("garbage gave %+v, want nothing", got)
	}
}
func TestReadOnlySheetsListResourcesWithWhatIsLeft(t *testing.T) {
	facts := sharedResources([]pages.ClassResource{
		{ID: testRageID, Name: "Rage", Max: 3, Used: 1},
		{ID: testBlankID, Name: "Unset", Max: 0},
		{ID: testSorceryID, Max: 12, Used: 12},
	})
	want := []pages.SharedFact{{Label: "Rage", Value: "2 of 3 left"}, {Label: "Resource", Value: "0 of 12 left"}}
	if !reflect.DeepEqual(facts, want) {
		t.Errorf("facts = %+v, want %+v", facts, want)
	}
}
