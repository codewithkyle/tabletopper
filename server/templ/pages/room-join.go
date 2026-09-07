package pages

// THE PAGE A PLAYER LANDS ON, and it asks two questions: which table, and who
// are you bringing. It is a page rather than a dialog because it is one of the
// two things the homepage links to -- somebody arriving at the app to play,
// rather than somebody already inside it who wants a new thing -- and because a
// code can be pasted into a URL, which /rooms/join/{code} is for.
//
// THE FORM IS NOT RE-RENDERED ON A REFUSAL. It posts with the hx-status:422
// trio every other form in the app uses, so a rejected submission swaps the
// error block alone and leaves the form standing -- which means the code the
// player typed and the character they picked are still in the DOM they were
// typed into. That is why this struct carries no error field and no submitted
// values beyond the prefill: there is nothing for the server to put back.
//
// ALL OF THE REASONING ABOUT THIS PAGE LIVES IN THIS FILE, because room-join.templ
// cannot carry a comment -- see rooms.go for what a sentence in a .templ file
// costs.

// JoinRoomPanel is the error block this form owns. Every refusal the join can
// produce -- a malformed code, a character that is not yours, too many tries, a
// room that is not open, a room that is locked -- lands in it as a 422, so the
// player reads the reason above the field they typed into rather than in a
// dialog that takes the page away from them.
const JoinRoomPanel = "join-room"

// NoCharacterValue is what the character picker submits for "No character". It
// is the empty string rather than a sentinel like "none", because the empty
// string is what an unset <select> and an absent field both produce, so the
// handler has one case to read instead of three.
const NoCharacterValue = ""

// JoinRoomPageData is the form's starting state.
type JoinRoomPageData struct {
	// Code prefills the field, for the /rooms/join/{code} form of the page that
	// a pasted link lands on. It is never a join by itself: a GET that seated
	// somebody at a table would be a state change behind a link, which is the
	// same rule that put /logout on POST.
	Code string

	// Characters is the player's own roster, offered beside "No character". A
	// player may join before they have made one, so the picker never blocks the
	// form -- the character is what a pawn is spawned from later, and a table
	// full of people watching is a legitimate way to spend a session.
	Characters []JoinCharacterOption
}

// JoinCharacterOption is one entry in the picker: the id the form submits and
// the name the player reads. It is not queries.Character, because a select
// needs two strings and that struct is the whole sheet -- fifty columns to
// render an <option>, and a page that would have to change every time the sheet
// does.
type JoinCharacterOption struct {
	ID   string
	Name string
}
