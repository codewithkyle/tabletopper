package pages

import "github.com/a-h/templ"

// shellLayout is what one editor page can change about the shell around it.
// Every field's zero value is what the other four tabs already do, so the
// struct is `shellLayout{}` everywhere except the journal entry page.
//
// It is a struct and not two more parameters because both fields answer the
// same question -- "what is different about this tab?" -- and a call site
// reading `shellLayout{Fill: true}` says which of the two it means, while
// `editCharacterShell(id, tab, true, nil)` does not.
type shellLayout struct {
	// Fill gives the tab's content the whole height below the tabs instead of
	// stacking panels from the top and scrolling the column. The journal editor
	// wants it because a text box that grows as you type pushes its own bottom
	// edge off the screen; every other tab is a stack of panels that is taller
	// than the viewport by design.
	Fill bool

	// Actions renders beside the Back button in the page header. The journal
	// entry page puts its Save there: the entry autosaves on a debounce like
	// every other panel, but a journal save is deliberately silent (see
	// finishJournalEntry), and a writer who is never told anything has no way to
	// know the debounce is working. The button is not a second save path -- it
	// posts the same form to the same route -- it is the one that answers.
	Actions templ.Component
}

// panelFormID is the id a savingPanel's form carries, so a control OUTSIDE the
// form can name it in hx-include. The panel's error block is `errors-<panel>`
// and this is `panel-<panel>`, from the same word.
//
// Only the journal's Save button uses it today. It is on every panel anyway
// because a form that can only be reached from inside itself is a limitation
// nobody chose -- and because generating the id from the panel name is what
// keeps the two from drifting apart.
func panelFormID(panel string) string {
	return "panel-" + panel
}

// THE ELEVATION SCALE, which is three levels and is defined by the two consts
// at the top of sheet-section.templ.
//
// They live in the .templ file rather than here because Tailwind scans
// `server/templ/**/*.templ` and nothing else. A class name written in a .go
// file is never emitted, so a const holding one has to sit in a scanned file
// even though it is ordinary Go and nothing in it is markup.
//
// L0, the desk. The grid paper in core.css. Nothing opts into it; it is what is
// behind everything.
//
// L1, raised -- surfacePanel. The bar, every sheet panel, the character and
// asset cards, the modals, the user badge. A 1px hairline, the panel fill, and
// --shadow-panel, which is the per-theme two-layer drop shadow declared in
// css/app.css. It replaced `card-border` plus `shadow-sm`: card-border reads
// --border, which is pinned to 2px so a card would not change thickness with
// the OS theme, and 2px of base-300 around every panel was doing the separating
// that a shadow should do.
//
// L2, inset -- surfaceInset. Everything that sits INSIDE a panel: bonus rows,
// attack rows, inventory rows, spell rows, the spell-slot and prepared-spell
// lists, equipped items, feature rows, journal cards, and the two derived
// readouts. A fill at 5% of base-content, no border, no shadow.
//
// L2 IS THE POINT OF THE SCALE. Every one of those used to carry the same
// `rounded-field border-2 border-base-300` as the panel around it, so a row in
// a panel was drawn on the same plane as the panel -- boxes inside boxes,
// nothing containing anything. Recessing them is what makes a panel read as a
// container, and it is why the borders came off rather than being thinned.
//
// The three modal boxes in layouts/base.templ write surfacePanel's classes out
// by hand. layouts cannot import pages -- pages imports layouts -- and the
// alternative was moving the scale into the package that owns the page shell
// rather than the one that owns every component using it.
