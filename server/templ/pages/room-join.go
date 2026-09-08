package pages

// THE PAGE A PLAYER LANDS ON, and it asks two questions: which table, and who
// are you bringing. It is a page rather than a dialog because it is one of the
// two things the homepage links to -- somebody arriving at the app to play,
// rather than somebody already inside it who wants a new thing -- and because a
// code can be pasted into a URL, which /rooms/join/{code} is for.
//
// A CHARACTER IS REQUIRED. It was optional at first, on the reasoning that a
// table full of people watching is a legitimate way to spend a session; that is
// still true and is not what this form is for. Everything downstream of a join
// assumes a character: the pawn a player spawns comes from one, the player list
// draws people by the character they brought, and a seat with nothing in it is
// a row that reads as a bug. Somebody who only wants to watch can be given the
// room by the GM.
//
// SO THE PAGE HAS TWO STATES, and the one with no form is the interesting one:
// an account with no characters yet cannot answer this question, and the honest
// thing to show them is where characters are made rather than a picker with
// nothing in it and a button that cannot work.
//
// THE FORM IS NOT RE-RENDERED ON A REFUSAL. It posts with the hx-status:422
// trio every other form in the app uses, so a rejected submission swaps the
// error block alone and leaves the form standing -- which means the code the
// player typed and the character they picked are still in the DOM they were
// typed into. That is why this struct carries no error field and no submitted
// values beyond the prefill: there is nothing for the server to put back.
//
// THE CODE FIELD IS THE DAISYUI OTP COMPONENT, which is a <label> holding one
// real input and four empty spans -- the spans are the boxes, the input sits
// over them with the letter spacing that lines the characters up with the gaps.
// The input is the field: it carries the name, the value, the maxlength and the
// pattern, and it is what the browser validates and submits. Clicking a box
// focuses it because the label wraps it, which is the only reason the
// component's `pointer-events: none` on the input is not a bug.
//
// IT IS autocomplete="off" AND NOT "one-time-code", which is what DaisyUI's own
// example carries. A room code is not a one-time code: it lives for as long as
// the room is open and it arrives over voice chat, so offering to fill it from
// the last SMS somebody received would be a suggestion that is always wrong.
// There is no inputmode either, because the alphabet is letters and digits and
// a numeric keypad cannot type it.
//
// THE HINT UNDER IT HANGS OFF THE LABEL, NOT OFF THE INPUT. Every other field
// in this app puts `peer` on the control and reads `peer-user-invalid` on the
// sibling that follows it, which cannot work here -- the input is a child of
// the label, so the hint is not its sibling. `peer` goes on the label and the
// hint asks `peer-has-[:user-invalid]` instead: the same question, one level
// out.
//
// ALL OF THE REASONING ABOUT THIS PAGE LIVES IN THIS FILE, because room-join.templ
// cannot carry a comment -- see rooms.go for what a sentence in a .templ file
// costs.

// JoinRoomPanel is the error block this form owns. Every refusal the join can
// produce -- a malformed code, a character that is not yours or not chosen, too
// many tries, a room that is not open, a room that is locked -- lands in it as
// a 422, so the player reads the reason above the field they typed into rather
// than in a dialog that takes the page away from them.
const JoinRoomPanel = "join-room"

// NoCharacterValue is what the picker's placeholder option carries. It is the
// empty string rather than a sentinel like "none", because the empty string is
// what an unset <select> and an absent field both produce, so the handler has
// one case to read instead of three.
//
// The option is `disabled`, so a player cannot choose it back after choosing a
// character, and `required` on the select is what stops the form submitting
// while it is still selected. Neither is the check: the handler refuses an
// empty value, because a form is markup and markup is a suggestion.
const NoCharacterValue = ""

// JoinRoomPageData is the form's starting state.
type JoinRoomPageData struct {
	// Code prefills the field, for the /rooms/join/{code} form of the page that
	// a pasted link lands on. It is never a join by itself: a GET that seated
	// somebody at a table would be a state change behind a link, which is the
	// same rule that put /logout on POST.
	Code string

	// Characters is the player's own roster, and an empty one is the page's
	// other state rather than an empty picker -- see above.
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
