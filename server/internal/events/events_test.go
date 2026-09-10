package events

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// browserModule is the JavaScript twin of this package, relative to it.
const browserModule = "../../public/js/events.js"

// THE PIN. The browser module and this package have to export the same names
// with the same values, and no other script in either bundle may spell one of
// them out -- a literal elsewhere is exactly the second copy this exists to
// remove, and it is the one that drifts.
func TestTheBrowserAgreesOnEveryEventName(t *testing.T) {
	src, err := os.ReadFile(browserModule)
	if err != nil {
		t.Fatalf("the browser module is missing: %v", err)
	}

	exported := regexp.MustCompile(`(?m)^export const ([A-Z_]+) = "([^"]+)";$`)
	found := map[string]string{}
	for _, m := range exported.FindAllStringSubmatch(string(src), -1) {
		found[m[1]] = m[2]
	}

	for name, value := range All {
		got, ok := found[name]
		if !ok {
			t.Errorf("%s = %q is not exported by %s", name, value, browserModule)

			continue
		}
		if got != value {
			t.Errorf("%s is %q in Go and %q in the browser", name, value, got)
		}
	}
	for name, value := range found {
		if _, ok := All[name]; !ok {
			t.Errorf("%s = %q is exported by the browser and unknown to Go", name, value)
		}
	}
}

// NO SCRIPT SPELLS ONE OUT. The two bundles are searched for every value as a
// quoted string; the only file allowed to hold one is the module itself, and a
// test file, which is asserting rather than raising.
func TestNoScriptSpellsAnEventNameOut(t *testing.T) {
	roots := []string{"../../public/js", "../../js/room"}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			base := filepath.Base(path)
			if base == "events.js" || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".d.ts") {
				return nil
			}
			if !strings.HasSuffix(base, ".js") && !strings.HasSuffix(base, ".ts") {
				return nil
			}

			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			for name, value := range All {
				for _, quoted := range []string{`"` + value + `"`, `'` + value + `'`} {
					if strings.Contains(string(src), quoted) {
						t.Errorf("%s spells %s out as %s; import it from events.js", path, name, quoted)
					}
				}
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// AND NEITHER DOES A ROOM PAGE. The fragments' hx-trigger attributes are built
// from these constants; a literal in a .templ file would also be a Tailwind
// class candidate, which is the other reason the names are Go.
func TestNoRoomPageSpellsAPanelEventOut(t *testing.T) {
	matches, err := filepath.Glob("../../templ/pages/room*")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_templ.go") || strings.HasSuffix(base, "_test.go") {
			continue
		}

		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		for name, value := range All {
			if !strings.HasPrefix(value, "room:") && !strings.HasPrefix(value, "window:") {
				continue
			}
			if strings.Contains(string(src), `"`+value) {
				t.Errorf("%s spells %s out; use events.%s", path, value, name)
			}
		}
	}
}
