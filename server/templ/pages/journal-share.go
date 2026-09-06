package pages

// The share dialog, in the content modal, in one of two states: the entry is
// not shared and the form offers to share it, or it is and the link is there to
// copy or revoke. One type and one exported component serve both, because the
// three routes behind them -- open, create, revoke -- all answer with whichever
// state the entry is now in, and a dialog that swapped between two components
// would need every caller to decide which.
//
// LINK IS WHAT DECIDES. There is no Shared bool: a share that exists has a URL
// and one that does not has nothing to render, so a second field could only
// ever disagree with the first.
//
// THE FORM'S OWN STATE IS NOT IN HERE, and that is the hx-status:422 on it
// rather than an omission. A rejected submission swaps the error block alone
// and leaves the form standing, so the toggles the reader flipped and the
// number they typed are still in the DOM they were typed into -- which is both
// better than re-rendering them from a struct and less code than doing so.

// journalShareID is the swap target for everything the dialog does. It is a
// constant because three places name it -- the container, and the hx-target on
// each of the two actions inside it -- and a target that has drifted from its
// id fails silently: htmx finds nothing to swap and the button stops working
// with the dialog still open in front of it.
const journalShareID = "journal-share"

// JournalSharePanel is the error block the create form owns, so a rejection
// here takes the 422 path every other form in the app takes.
const JournalSharePanel = "journal-share"

// ShareDefaultDays is what the days box starts at, and it is a default rather
// than a placeholder for one reason: the box is inside a toggle, and HTML has
// no way to make a field required only while its toggle is on. A box that
// started empty would let a reader ask for an expiry without saying when, which
// the server would then have to refuse for no reason they could see. A week
// means flipping the toggle is already a complete answer.
const ShareDefaultDays = "7"

// JournalShareData is the dialog.
type JournalShareData struct {
	CharacterID string
	EntryID     string

	// Link is the whole URL, scheme and host included, because it is a value
	// to be copied out of the page and pasted into a chat window rather than a
	// path for this app to follow. The controller builds it from the request
	// that asked for it, which is the only thing that knows what host the
	// reader reached this app on.
	Link string

	// Expires is when the link stops working; the zero value means never.
	// Expired says that instant has already passed. The row outlives its
	// expiry by a day -- the sweeper leaves it so an owner opening the dialog
	// the next morning sees a link that expired rather than an entry that
	// appears never to have been shared -- so the dialog has to be able to say
	// "this one is dead" instead of offering a URL that answers 404.
	Expires Timestamp
	Expired bool

	// Protected is whether a password stands in front of the link. The
	// password itself is never rendered and cannot be: it is bcrypt in the row,
	// and was last legible in the request that set it.
	Protected bool
}
