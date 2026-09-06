package pages

// The share dialog, in the content modal, in one of two states: the thing is
// not shared and the form offers to share it, or it is and the link is there to
// copy or revoke. One type and one exported component serve both states AND
// both kinds of share, because the six routes behind them -- open, create and
// revoke, for an entry and for a sheet -- all answer with whichever state the
// thing is now in, and a dialog that swapped between components would need
// every caller to decide which.
//
// WHAT A JOURNAL SHARE AND A CHARACTER SHARE DIFFER BY IS THREE STRINGS. The
// heading, the sentence under it and the URL its buttons work against; the
// expiry, the password, the link, the facts and the revoke are the same markup
// either way. So the difference is data on this struct rather than a second
// copy of the form, which is also what stops the two drifting the first time
// one of them gains a field.
//
// LINK IS WHAT DECIDES THE STATE. There is no Shared bool: a share that exists
// has a URL and one that does not has nothing to render, so a second field
// could only ever disagree with the first.
//
// THE FORM'S OWN STATE IS NOT IN HERE, and that is the hx-status:422 on it
// rather than an omission. A rejected submission swaps the error block alone
// and leaves the form standing, so the toggles the reader flipped and the
// number they typed are still in the DOM they were typed into -- which is both
// better than re-rendering them from a struct and less code than doing so.

// shareDialogID is the swap target for everything the dialog does. It is a
// constant because three places name it -- the container, and the hx-target on
// each of the two actions inside it -- and a target that has drifted from its
// id fails silently: htmx finds nothing to swap and the button stops working
// with the dialog still open in front of it.
const shareDialogID = "share-dialog"

// ShareDialogPanel is the error block the create form owns, so a rejection here
// takes the 422 path every other form in the app takes. One name for both kinds
// of share, because one dialog is open at a time and the block is inside it.
const ShareDialogPanel = "share-dialog"

// ShareDefaultDays is what the days box starts at, and it is a default rather
// than a placeholder for one reason: the box is inside a toggle, and HTML has
// no way to make a field required only while its toggle is on. A box that
// started empty would let a reader ask for an expiry without saying when, which
// the server would then have to refuse for no reason they could see. A week
// means flipping the toggle is already a complete answer.
const ShareDefaultDays = "7"

// ShareDialogData is the dialog.
type ShareDialogData struct {
	// Heading and Blurb are the two sentences that name what is being shared.
	// The controller writes them because it is the half that knows, and they
	// are fields rather than a kind enum the template switches on -- a switch
	// here would put the wording of both shares in the markup and mean a third
	// share could not be added without editing it.
	Heading string
	Blurb   string

	// Action is the resource URL the dialog's two buttons work against: the
	// create POSTs to it, the revoke DELETEs it. One field and not two, because
	// they are the same URL and a pair could disagree -- a dialog whose revoke
	// pointed at another share's row is the one mistake this shape rules out.
	Action string

	// Link is the whole URL, scheme and host included, because it is a value
	// to be copied out of the page and pasted into a chat window rather than a
	// path for this app to follow. The controller builds it from the request
	// that asked for it, which is the only thing that knows what host the
	// reader reached this app on.
	Link string

	// Expires is when the link stops working; the zero value means never.
	// Expired says that instant has already passed. The row outlives its
	// expiry by a day -- the sweeper leaves it so an owner opening the dialog
	// the next morning sees a link that expired rather than a thing that
	// appears never to have been shared -- so the dialog has to be able to say
	// "this one is dead" instead of offering a URL that answers 404.
	Expires Timestamp
	Expired bool

	// Protected is whether a password stands in front of the link. The
	// password itself is never rendered and cannot be: it is bcrypt in the row,
	// and was last legible in the request that set it.
	Protected bool
}

// The two URLs the Share buttons carry in data-modal-open, which
// content-modal.js turns into the modal:open event every other caller
// dispatches inline.
//
// THEY ARE ATTRIBUTES RATHER THAN LINES OF SCRIPT BECAUSE THEY ARE COMPUTED.
// templ reads hx-on: as an event handler and will only take a
// templ.ComponentScript there, so a URL built from an id cannot be written into
// one -- the static dispatches elsewhere in the app are literals, which is why
// they can be.
//
// No id needs escaping: every one is a ULID the controller parsed, and 26
// characters of Crockford base32 cannot carry a quote, an ampersand or a space.

func journalShareDialogURL(characterID, entryID string) string {
	return "/fragment/character/journal-share?character=" + characterID + "&entry=" + entryID
}

func characterShareDialogURL(characterID string) string {
	return "/fragment/character/share?character=" + characterID
}
