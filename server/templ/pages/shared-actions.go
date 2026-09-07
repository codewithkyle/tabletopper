package pages

// The row of things a reader can do with a shared page, which is the one part of
// a shared page that is not the same for everybody.
//
// IT IS ONE TYPE AND ONE COMPONENT FOR BOTH SHARED PAGES, for the reason
// ShareDialogData is one type for three kinds of share: a monster's row and a
// character's differ by which buttons are in them, and everything else about the
// row -- where it sits, how it wraps, what it looks like -- is the same. A
// second copy would be the thing that drifts.
//
// EVERY FIELD IS A URL OR A SENTENCE THE CONTROLLER WROTE. Nothing here is a
// flag the markup switches on, so a third action can be added without this
// component learning what a monster is -- and the prose lives in Go rather than
// in the .templ, which is where prose belongs in this app: a .templ is a
// Tailwind source and every word in one is a class-name candidate.
type SharedActions struct {
	// Blurb explains an action that needs explaining, and only the import does.
	// Empty is the ordinary case and renders the row as a toolbar with the
	// buttons alone.
	Blurb string

	// Export is the Markdown download, and it is the one action every reader of
	// every shared page gets -- signed in or not, owner or stranger. The file
	// is what the page shows, so there is nobody who can read the page and
	// should not be able to keep it.
	Export string

	// Import is the copy button, set only on a monster's page and only for a
	// signed-in reader who is not the owner. SignIn stands in its place for a
	// reader with no account, because a button that answered with a sign-in
	// page would be a worse way of saying the same thing.
	//
	// AT MOST ONE OF THE TWO IS SET, and both being empty is the owner reading
	// their own link: importing their own monster would hand them a duplicate
	// they did not ask for.
	Import string
	SignIn string
}
