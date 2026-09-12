package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func debugFragments() map[string]templ.Component {
	return map[string]templ.Component{
		"renderer": RoomDebugRenderer(),
		"events":   RoomDebugEvents(),
		"state":    RoomDebugState(),
		"server":   RoomDebugServer(testDebugServer()),
	}
}
func testDebugServer() RoomDebugServerData {
	return RoomDebugServerData{
		RoomID: "01BX5ZZKBKACTAV9WEVGEMMVT0",
		Live:   true,
		Groups: []RoomDebugGroup{{Label: "Actor", Rows: []RoomDebugRow{
			{Label: "Connections", Value: "1 of 64"},
			{Label: "Inbox", Value: "60 of 64", Warn: true},
		}}},
		PerUser:    []RoomDebugConn{{User: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Count: 4, Over: true}},
		Coalescing: []string{"pawn.drag:01PAWN"},
	}
}
func debugPanel(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "js", "room", "debug", "panel.ts"))
	if err != nil {
		t.Fatalf("read panel.ts: %v", err)
	}
	return string(source)
}

var debugKeys = regexp.MustCompile(`data-debug="([a-z0-9-]+)"`)

func TestEveryDebugReadoutIsOneThePanelWrites(t *testing.T) {
	panel := debugPanel(t)
	for pane, component := range debugFragments() {
		for _, match := range debugKeys.FindAllStringSubmatch(markup(t, component), -1) {
			key := match[1]
			bare := key + ":"
			quoted := `"` + key + `":`
			if !strings.Contains(panel, bare) && !strings.Contains(panel, quoted) {
				t.Errorf("the %s pane shows %q and panel.ts never writes it, so the row stays blank", pane, key)
			}
		}
	}
}
func TestEveryHookThePanelReachesForIsInTheMarkup(t *testing.T) {
	rendered := map[string]string{}
	for pane, component := range debugFragments() {
		rendered[pane] = markup(t, component)
	}
	for pane, hooks := range map[string][]string{
		"renderer": {
			"data-debug-graph",
			"data-debug-stress-count",
			"data-debug-stages",
			"data-debug-stage",
			"data-debug-stage-name",
			"data-debug-stage-cost",
			`data-debug-action="benchmark"`,
			`data-debug-action="stress"`,
			`data-debug-action="lose-context"`,
			`data-debug-action="timing"`,
		},
		"events": {
			"data-debug-events",
			"data-debug-row",
			"data-debug-row-line",
			"data-debug-row-json",
			"data-debug-types",
			"data-debug-type",
			"data-debug-type-name",
			"data-debug-type-count",
			"data-debug-filter",
			"data-debug-form",
			"data-debug-input",
			`data-debug-action="pause"`,
			`data-debug-action="copy"`,
			`data-debug-action="clear"`,
			"data-debug-skip-count",
			"data-debug-latency",
			"data-debug-jitter",
			"data-debug-loss",
			`data-debug-action="skip"`,
			`data-debug-action="reconnect"`,
			`data-debug-action="impair"`,
		},
		"state": {
			"data-debug-players",
			"data-debug-slices",
			"data-debug-slice",
			"data-debug-slice-name",
			"data-debug-slice-detail",
			`data-debug-action="check"`,
		},
	} {
		for _, hook := range hooks {
			if !strings.Contains(rendered[pane], hook) {
				t.Errorf("the %s pane has no %s, so the panel would paint nothing into it", pane, hook)
			}
		}
	}
}
func TestEachPaneIsMarkedOnceAndOnlyForItself(t *testing.T) {
	for pane, component := range debugFragments() {
		rendered := markup(t, component)
		mine := `data-debug-pane="` + pane + `"`
		if strings.Count(rendered, mine) != 1 {
			t.Errorf("the %s pane carries its marker %d times, want once", pane, strings.Count(rendered, mine))
		}
		if strings.Count(rendered, "data-debug-pane=") != 1 {
			t.Errorf("the %s fragment carries another pane's marker", pane)
		}
	}
}
func TestEveryRowTemplateSitsBesideTheListItFills(t *testing.T) {
	for _, pair := range []struct{ pane, list, template string }{
		{"events", "data-debug-events", "data-debug-row"},
		{"events", "data-debug-types", "data-debug-type"},
		{"state", "data-debug-slices", "data-debug-slice"},
		{"renderer", "data-debug-stages", "data-debug-stage"},
	} {
		rendered := markup(t, debugFragments()[pair.pane])
		if !strings.Contains(rendered, "<template "+pair.template) {
			t.Errorf("%s is not a template, so the panel cannot clone a row for %s", pair.template, pair.list)
		}
	}
}
func TestTheDebugPanesWriteNoClassNamesFromScript(t *testing.T) {
	panel := debugPanel(t)
	for _, banned := range []string{".className", "classList.add", "classList.remove", "classList.toggle"} {
		if strings.Contains(panel, banned) {
			t.Errorf("panel.ts uses %s; server/js is not a Tailwind source, so the class would never be emitted", banned)
		}
	}
}
func TestTheDebugKeysAreUniqueWithinAPane(t *testing.T) {
	for pane, component := range debugFragments() {
		seen := map[string]int{}
		for _, match := range debugKeys.FindAllStringSubmatch(markup(t, component), -1) {
			seen[match[1]]++
		}
		repeated := []string{}
		for key, count := range seen {
			if count > 1 {
				repeated = append(repeated, key)
			}
		}
		sort.Strings(repeated)
		if len(repeated) > 0 {
			t.Errorf("the %s pane repeats %v, so two rows would show the same value", pane, repeated)
		}
	}
}
func TestTheServerPaneRefetchesItselfWithoutRaisingTheLoadingBar(t *testing.T) {
	rendered := markup(t, RoomDebugServer(testDebugServer()))
	for _, want := range []string{
		`id="room-debug-server"`,
		`hx-get="/fragment/room/debug/server?room=01BX5ZZKBKACTAV9WEVGEMMVT0"`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
		"data-quiet",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the server pane is missing %s", want)
		}
	}
}
func TestTheServerPaneWarnsOnTheRowsThatMatter(t *testing.T) {
	rendered := markup(t, RoomDebugServer(testDebugServer()))
	if !strings.Contains(rendered, `class="truncate font-mono text-warning">60 of 64`) {
		t.Error("a backed-up inbox is not called out")
	}
	if !strings.Contains(rendered, `class="truncate font-mono">1 of 64`) {
		t.Error("an ordinary row should not be dressed as a warning")
	}
	if !strings.Contains(rendered, `text-warning">01ARZ3NDEKTSV4RRFFQ69G5FAV`) {
		t.Error("a user at the connection cap is not called out")
	}
}
func TestAnUnloadedRoomSaysSoRatherThanShowingZeroes(t *testing.T) {
	rendered := markup(t, RoomDebugServer(RoomDebugServerData{RoomID: "01BX5ZZKBKACTAV9WEVGEMMVT0"}))
	if !strings.Contains(rendered, "not loaded in the server") {
		t.Error("an unloaded room should say so; every counter would otherwise read zero and look like a bug")
	}
	if strings.Contains(rendered, "Goroutines") {
		t.Error("an unloaded room has no actor to report on")
	}
}
