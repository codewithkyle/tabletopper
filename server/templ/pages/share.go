package pages

import "strings"

// The shared journal entry, as a stranger sees it: a banner naming the
// character, the entry's title, and the entry.
//
// THE TYPE CARRIES WHAT THE PAGE SHOWS AND NOTHING ELSE -- no ids, no character
// sheet, no owner -- so a field added to the character row later cannot arrive
// here by being part of a struct that was passed through. The controller reads
// six columns and fills six fields. SharedCharacterSheet makes the same promise
// for the other shared page and keeps it a different way, because that one shows
// most of a sheet rather than five values off it.
//
// There is no link back into the app, deliberately. A reader who followed a link
// to one entry has been given one entry, and a header offering the character's
// sheet would be offering something the share does not cover -- which stays true
// now that a sheet can be shared, because that is a second link with its own
// expiry and its own password.

// SharedCharacter is the banner: enough to know whose diary this is, and no
// more of the sheet than that takes.
type SharedCharacter struct {
	Name    string
	Level   string
	Classes string
	Race    string

	// Avatar is the share-scoped URL of the portrait, empty when the character
	// has none. It is never /assets/images/{id}: that route needs a session,
	// so on this page it would render as a broken image for every reader.
	//
	// The initial underneath it is not a field: characterInitial already
	// derives one from a name for the character cards, and the letter shows
	// through whether or not there is a picture on top of it -- which is what
	// a reader sees if the object has gone from the bucket, since a shared
	// page has nothing to retry with.
	Avatar string
}

// SharedJournalData is the whole page.
//
// BODY IS HTML AND EVERY OTHER FIELD IS TEXT. It is the one string in the app
// written to a response without escaping, which is why it is produced by
// internal/markdown and by nothing else: that package renders with goldmark's
// raw-HTML support off, so what comes back holds no markup the writer put
// there -- see its package comment for the three defences that rests on.
type SharedJournalData struct {
	Character SharedCharacter
	Title     string
	Body      string
}

// ShareLockedData is the password gate, and it is one gate for both kinds of
// share.
//
// IT NAMES NOTHING. Not the character, not an entry's title, not who shared it,
// and not even which of the two it is guarding -- the password is there to keep
// the thing from being read by whoever finds the link, and a gate that announced
// what it stood in front of would have given away part of the answer before
// asking the question. That is why its wording is about a link rather than about
// an entry: the gate cannot say, because it is not told.
type ShareLockedData struct {
	// Action is the URL the form posts to, which is the share's own URL. The
	// controller supplies it rather than the template rebuilding it from a
	// token, so the token appears in one place.
	Action string

	// Problem is the sentence shown after a wrong password, empty on the first
	// ask. There is only ever one, and it never says which half was wrong,
	// because there is only one half.
	Problem string
}

// shareEntryTitle covers an entry that was shared before it was named. The
// editor's list has the same hole and answers it the same way; the difference
// is that here the fallback is also the page's <title>, so a browser tab
// showing nothing at all is what it prevents.
func shareEntryTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Untitled entry"
	}

	return title
}

// ShareTitle is the <title> for a shared page: the entry's name and nothing
// about the app around it.
func ShareTitle(title string) string {
	return shareEntryTitle(title) + " | Tabletopper"
}
