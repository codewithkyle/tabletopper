package pages

// THE PIECES EVERY ASSET CARD HAS, whatever kind of asset it is showing.
//
// A map, a token and an avatar are three different rows with three different
// stories -- one is a tile pyramid with a background job behind it, the other
// two are a single stored image -- but the bottom two thirds of their cards are
// the same control three times: a name you can type over, the uploaded filename
// as a chip, a Replace and a Delete.
//
// They are shared as two components taking a struct rather than as six loose
// string parameters, because a call reading `@assetCardControls(m.controls())`
// cannot get its arguments in the wrong order, and `@assetCardControls(id,
// url, url, field, accept, name)` can -- silently, into a card that posts a
// replacement at the delete route.
//
// THE STRUCTS ARE BUILT BY METHODS ON THE PAGE-DATA TYPES, never by the
// markup. That is where a URL is allowed to be assembled, and it is the same
// rule MapAsset.CardURL already follows: a path built in a .templ file is one
// nothing can point at from routes.go.

// nameBox is the name control on an asset card: a borderless box that looks
// like a heading until it is hovered, and saves a second after typing stops.
//
// IT IS A BARE INPUT AND NOT A FORM, which is what decides how it saves. There
// is nothing to submit and no error block to write into, so the response is a
// toast and hx-swap="none"; a name too long for the column is refused by the
// browser through MaxLength rather than by the server through a message that
// would have nowhere to go.
type nameBox struct {
	// ID is the element's own id. It is not used as a label target -- the box
	// is its own label -- but a stable id is what lets a swap replace the card
	// without the browser losing the caret.
	ID string
	// Value is the name as it stands.
	Value string
	// URL is the PATCH this box posts to, which is the asset's name resource.
	URL string
	// Field is the form field name the handler reads.
	Field string
	// MaxLength is AssetNameLimit as a string, because an attribute is text.
	MaxLength string
	// Size is the box's height and type scale: nameBoxRoomy or nameBoxTight,
	// both declared in asset-card.templ because a class named only in a .go
	// file is never built.
	//
	// IT IS A SLOT AND NOT A BOOLEAN because what varies is a pair of class
	// names, and class names have to be written somewhere Tailwind scans. The
	// tight one exists for the Avatars page and nothing else: a face wall is
	// eleven rems to a card, and a 24-pixel serif heading in a card that narrow
	// truncates every name to about ten characters, which is a heading that has
	// stopped naming anything.
	Size string
}

// cardControls is the strip along the bottom of an asset card: the filename,
// Replace, and Delete.
type cardControls struct {
	// FileName is the name of the file as it was uploaded, shown as a chip. It
	// is not the asset's name and is not editable -- it is how a row in the
	// bucket's ledger is recognised by a person looking at both.
	FileName string
	// ReplaceID joins the visible label to the file input hidden inside it.
	// The input is a zero-sized transparent box rather than display:none,
	// because a hidden input cannot be focused and a label for an unfocusable
	// control is not reachable by keyboard.
	ReplaceID string
	// ReplaceURL is the POST that overwrites the asset in place, and Field is
	// the form field it reads.
	ReplaceURL string
	Field      string
	// Accept is the file picker's filter. It is a hint and not a check -- the
	// handler inspects the bytes -- but it is what stops somebody choosing a
	// PDF and waiting for an upload that was never going to work.
	Accept string
	// DeleteURL is the DELETE, and ConfirmName is the asset as the confirm
	// dialog should name it, so the sentence reads "You are about to delete
	// The Sunless Citadel" rather than naming an id.
	DeleteURL   string
	ConfirmName string
}

// imageAccept is the file picker's filter on every image upload in the asset
// manager, and it is the same three everywhere because the same three decoders
// are registered in the handler -- see openImageUpload.
const imageAccept = "image/png, image/jpeg, image/webp"
