package layouts

import (
	"bytes"
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/buildinfo"

	"github.com/a-h/templ"
)

var localURL = regexp.MustCompile(`(?:src|href)="(/[^/"][^"]*)"`)

func TestEveryLocalAssetInTheLayoutsIsVersioned(t *testing.T) {
	prefix := "/v/" + buildinfo.Version() + "/"
	for name, c := range map[string]struct {
		layout templ.Component
		want   []string
	}{
		"the base layout": {
			layout: Base("Room", "/js/room.js", "/static/room.js"),
			want:   []string{"/css/app.css", "/js/toast.js", "/static/htmx.min.js", "/js/room.js", "/static/room.js"},
		},
		"the share layout": {
			layout: Share("Vex", false),
			want:   []string{"/css/app.css", "/css/journal.css"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := c.layout.Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			page := buf.String()
			for _, m := range localURL.FindAllStringSubmatch(page, -1) {
				if !strings.HasPrefix(m[1], prefix) {
					t.Errorf("%s is not versioned, so a deploy leaves the old copy cached", m[1])
				}
			}
			for _, path := range c.want {
				if !strings.Contains(page, `="`+Asset(path)+`"`) {
					t.Errorf("the layout does not load %s", Asset(path))
				}
			}
		})
	}
}
func TestARelativeImportResolvesToTheURLTheLayoutLoads(t *testing.T) {
	importer, err := url.Parse("https://tabletopper.test" + Asset("/js/modal.js"))
	if err != nil {
		t.Fatal(err)
	}
	specifier, err := url.Parse("./toast.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := importer.ResolveReference(specifier).RequestURI(); got != Asset("/js/toast.js") {
		t.Errorf("modal.js imports ./toast.js as %s but the layout loads %s, so the browser would run toast.js twice", got, Asset("/js/toast.js"))
	}
}
