package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) ResourceRowFragment(w http.ResponseWriter, r *http.Request) {
	renderFragment(w, r, pages.ResourceRowFragment(ulid.Make().String()))
}
func (a *App) ResourceCounterFragment(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id, ok := resourceID(query.Get("id"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	renderFragment(w, r, pages.ResourceCounter(resourceFromForm(query, id)))
}
func resourceID(raw string) (string, bool) {
	id, err := ulid.ParseStrict(raw)
	if err != nil || id.String() != raw {
		return "", false
	}
	return raw, true
}
func resourceFromForm(form url.Values, id string) pages.ClassResource {
	most := int(clamped(form.Get(pages.ResourceField(id, "max")), uint16(0), 0, pages.ResourceMaxLimit))
	used := len(form[pages.ResourceField(id, "spent")])
	if counted, ok := form[pages.ResourceField(id, "used")]; ok && len(counted) > 0 {
		used = int(clamped(counted[0], uint16(0), 0, pages.ResourceMaxLimit))
	}
	return pages.ClassResource{
		ID:   id,
		Name: strings.TrimSpace(form.Get(pages.ResourceField(id, "name"))),
		Max:  most,
		Used: min(used, most),
	}
}
func resourcesFromForm(form url.Values) ([]pages.ClassResource, []string) {
	var problems []string
	rows := make([]pages.ClassResource, 0)
	seen := map[string]bool{}
	longName := false
	for _, raw := range form[pages.ResourceIDField] {
		id, ok := resourceID(raw)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		resource := resourceFromForm(form, id)
		if resource.Name == "" && resource.Max == 0 {
			continue
		}
		if len([]rune(resource.Name)) > pages.ResourceNameLimit && !longName {
			longName = true
			problems = append(problems, "Resource names must be "+strconv.Itoa(pages.ResourceNameLimit)+" characters or fewer.")
		}
		rows = append(rows, resource)
	}
	if len(rows) > pages.ResourceRowLimit {
		problems = append(problems, "A character can track at most "+strconv.Itoa(pages.ResourceRowLimit)+" resources.")
	}
	return rows, problems
}
func parseResources(raw json.RawMessage) []pages.ClassResource {
	if len(raw) == 0 {
		return nil
	}
	var rows []pages.ClassResource
	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Warn("invalid class resources payload; defaulting", "error", err)
		return nil
	}
	kept := rows[:0]
	for _, row := range rows {
		if _, ok := resourceID(row.ID); !ok {
			continue
		}
		row.Max = min(max(row.Max, 0), pages.ResourceMaxLimit)
		row.Used = min(max(row.Used, 0), row.Max)
		kept = append(kept, row)
	}
	return kept
}
func sharedResources(rows []pages.ClassResource) []pages.SharedFact {
	facts := make([]pages.SharedFact, 0, len(rows))
	for _, row := range rows {
		if row.Max == 0 {
			continue
		}
		facts = append(facts, pages.SharedFact{Label: row.Label(), Value: row.Left()})
	}
	return facts
}
