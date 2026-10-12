package layouts

import (
	"context"

	"tabletopper/internal/buildinfo"
	"tabletopper/internal/session"

	"github.com/a-h/templ"
)

func themeAttrs(ctx context.Context) templ.Attributes {
	palette := session.FromContext(ctx).Prefs.Theme.Palette()
	if palette == "" {
		return nil
	}
	return templ.Attributes{"data-theme": palette}
}
func Asset(path string) string {
	return "/v/" + buildinfo.Version() + path
}
