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
