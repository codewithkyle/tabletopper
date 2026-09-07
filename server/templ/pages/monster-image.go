package pages

// The one control that puts a picture on a monster, and the reasoning behind
// it, because monster-image.templ cannot carry a comment of any kind --
// Tailwind reads every .templ file as text and takes a class-name candidate
// from anything word-shaped it finds.
//
// IT IS ONE COMPONENT RENDERED IN THREE PLACES: the manual card, the editor's
// bar, and the reply the upload answers with. That last one is why it is a
// component at all. A picture can be set from either page, both post to the
// same URL, and the handler has to answer with something both can swap --
// so what it answers with is the control itself, and each page swaps the
// element it already had.
//
// THE CONTROL CARRIES NO SIZE. The card draws it at 3.5rem and the editor bar
// at 2.75rem, and if the size lived in here the handler would have to know
// which page asked in order to answer. Instead the caller wraps it in a box of
// the width it wants and everything inside is a percentage of that, so the
// swap replaces the control and leaves the box that sizes it on the page.

// MonsterImage is what the control needs, which is less than either page has:
// the monster's id for the URL it posts to, the name for the alt text and the
// letter behind an empty circle, and the asset id when there is a picture.
type MonsterImage struct {
	MonsterID string
	Name      string
	// ImageID is empty when the monster has no picture yet.
	ImageID string
}

// Verb is the word on the control, which is the one part of it that changes
// with whether there is a picture yet. It is a method rather than two branches
// of markup because the control is otherwise identical in both states, and the
// character card's two copies of it have already drifted once.
func (m MonsterImage) Verb() string {
	if m.ImageID == "" {
		return "Upload"
	}

	return "Replace"
}

// monsterImageInputID is the id the label points its `for` at. The two have to
// agree or the click does nothing, so they are generated from one place.
//
// It is per monster because the manual renders one control per card, and two
// inputs sharing an id would send every upload on the page to whichever came
// first.
func monsterImageInputID(monsterID string) string {
	return "monster-image-" + monsterID
}

// newMonsterImageInputID is the id for the create dialog's picker, where there
// is no monster to name yet. There is only ever one of these on screen.
//
// THE DIALOG'S PICKER IS NOT MonsterImageControl AND CANNOT BE. The control
// above posts the moment a file is chosen, because the monster it belongs to
// already exists; the dialog has nothing to post to yet, so its file input is
// an ordinary field of the create form and travels with the name. That is what
// makes the form multipart.
//
// It shows the chosen file before the form is sent, which is the one piece of
// this feature that has to be done in the browser -- the server has not seen
// the bytes yet. The handler is written inline rather than in
// server/public/js because that directory is not a Tailwind source: a class
// name written in there is never emitted, so the classes it toggles would not
// exist. It toggles the `hidden` attribute instead, which Tailwind's preflight
// backs with `display: none !important` and so wins over the display utilities
// the preview already carries.
const newMonsterImageInputID = "new-monster-image"
