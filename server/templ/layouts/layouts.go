// Package layouts holds the two page shells. Their reasoning lives here rather
// than beside them, because a comment in a .templ file is read by the Tailwind
// scanner as a list of class-name candidates -- see the ban in CLAUDE.md.
//
// Base is every page behind a session. It loads htmx, the three dialogs and the
// modules that drive them, and it centres its content, because every page it
// wraps is a card or a grid of them sized to the viewport.
//
// Share is the one page in front of no session at all: a journal entry someone
// handed out a link to. It is a separate shell rather than a flag on Base for
// two reasons, and the smaller one is that it needs none of that machinery --
// nothing on it posts, nothing swaps, and there is no dialog to open, so
// loading htmx and six modules would be six requests spent on a document that
// only has to be read.
//
// THE LARGER REASON IS THAT IT IS PUBLIC. Base is written for a reader who is
// signed in, and every URL it can be made to emit is one the app would answer
// for that reader. This shell renders a stranger's markdown next to a
// stranger's portrait for someone the app knows nothing about, and keeping the
// two apart means a control added to the signed-in layout later cannot arrive
// on the shared page by inheritance.
//
// The robots meta is the noindex half of what the handler also sends as a
// header, and it is in both places on purpose: an unlisted link that a crawler
// files away is not unlisted, and the header is what a fetcher that never
// parses the body sees.
//
// core.css fixes html and body at the viewport with overflow hidden, so the
// scrolling container has to be inside them -- the same shape every page under
// Base uses, where the card grid rather than the document is what scrolls.
package layouts

import (
	"context"

	"tabletopper/internal/session"

	"github.com/a-h/templ"
)

// themeAttrs is the data-theme attribute Base puts on <html>.
//
// SYSTEM RENDERS NO ATTRIBUTE AT ALL, which is what makes it work. app.css
// registers coffee with DaisyUI's --prefersdark flag, so the plugin emits it
// under `@media (prefers-color-scheme: dark) { :root:not([data-theme]) }` and
// caramellatte under `:where(:root)`. An absent attribute is therefore the OS
// preference, live, following a switch at dusk with no JavaScript; a present
// one outranks the media query through that :not(). None of this needs a line
// of CSS written for it -- the guard was put there for exactly this.
//
// It is also why the theme is server-rendered rather than applied on load. A
// class or attribute set by a script runs after the first paint, so the page
// flashes the wrong palette; written into the markup, there is nothing to
// flash.
//
// The context is the request's, and a request with no session -- the signed-out
// homepage -- reads the zero value, whose theme is system. That is the right
// answer for a stranger, and it is why this takes a context rather than an
// argument threaded through thirteen call sites of Base.
func themeAttrs(ctx context.Context) templ.Attributes {
	palette := session.FromContext(ctx).Prefs.Theme.Palette()
	if palette == "" {
		return nil
	}
	return templ.Attributes{"data-theme": palette}
}
