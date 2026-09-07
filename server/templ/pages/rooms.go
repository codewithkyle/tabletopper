package pages

// THE GM'S ROOMS, WHICH IS THE ROSTER'S SHAPE FOR TABLES. A room is a thing a
// GM keeps rather than a session they start, so this page is a list of them
// with a create dialog above it -- the same page the characters roster and the
// monster manual are, for the same reason: creation asks one question, and a
// page that asks one question is a dialog.
//
// ALL OF THE REASONING ABOUT THIS PAGE LIVES IN THIS FILE. rooms.templ cannot
// carry a comment of any kind: Tailwind reads every .templ file as text and
// takes a class-name candidate from every word in it, so an ordinary English
// sentence about "the room list" or "the lock status" emits a DaisyUI component
// family into the built stylesheet and nothing anywhere fails.
//
// THE CARD SHOWS THE CODE AND NOT THE URL. /rooms/{id} is where the GM goes and
// it is useless to a player -- the code is the whole of what gets somebody else
// to the table, so it is the thing on the card big enough to read out over
// voice chat.

// NewRoomPanel is the error block the create dialog owns, so a rejection there
// takes the 422 path every other form in the app takes.
const NewRoomPanel = "new-room"

// RoomNameLimit mirrors rooms.name, which is VARCHAR(128). It is here as well
// as in the handler because the field carries it as a maxlength: the browser
// refusing the 129th character is a better answer than the server refusing the
// submission, and the server still refuses it for the client that ignored the
// attribute.
const RoomNameLimit = 128

// RoomsPageData is the whole page. It is a struct with one field rather than a
// bare slice because the page grows a filter or a count long before it grows a
// second list, and a slice cannot take either without every caller changing.
type RoomsPageData struct {
	Rooms []RoomSummary
}

// RoomSummary is one room as its card needs it, with every conversion already
// done -- the controller turns a nullable code column and a nullable closed_at
// into a string and a bool, so the markup asks no questions of the database's
// types.
//
// CODE AND CLOSED ARE NOT THE SAME FACT AND ARE NOT DERIVED FROM EACH OTHER.
// The row makes them agree -- closing NULLs the code -- but the card reads them
// separately, because "closed" is what the actions branch on and the code is
// what gets printed, and a card that inferred one from the other would be
// re-deriving a rule the database already enforces.
type RoomSummary struct {
	ID   string
	Name string

	// Code is the four characters a player types, and it is empty for a closed
	// room -- a closed room gave its code back, and printing the one it used to
	// have would be printing a code that now belongs to somebody else.
	Code string

	Locked bool
	Closed bool
}
