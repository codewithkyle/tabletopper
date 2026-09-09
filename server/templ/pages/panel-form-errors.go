package pages

// THE ERROR SLOT EVERY AUTOSAVING FORM ANSWERS WITH, and the reasoning for the
// component in panel-form-errors.templ, which cannot hold a comment of its own.
//
// IT IS ALWAYS RENDERED, EVEN WITH NOTHING TO SAY. The forms that use it post
// with hx-target="#errors-<panel>" and hx-swap="outerHTML", so the element has
// to be in the document before the first save or the reply lands nowhere. That
// is also why a save that WORKS answers an empty block rather than a 204:
// something has to clear the message the previous attempt left on the screen.
//
// THE EMPTY ONE IS `hidden`, WHICH IS NOT COSMETIC. Its parents are flex and
// grid containers with a gap, and a zero-height child still takes a gap on each
// side of itself -- so an invisible element was pushing the top of every panel
// down by two gaps. `hidden` is display:none, which a gap skips entirely. The
// element is still there for htmx to swap.
//
// THE PANEL ARGUMENT IS NEVER USER INPUT. It goes into an id attribute and into
// the hx-target that names it, so a value out of a form field would be markup
// injection with a selector attached. Every caller passes either a constant or
// an id the handler has already parsed as a ULID.

// panelErrorsID is the block's own element id. A form aims at "#" + this, and a
// target that has drifted from its block fails silently -- htmx swaps nothing
// and the save looks like it worked -- so the prefix is written here once.
func panelErrorsID(panel string) string { return "errors-" + panel }
