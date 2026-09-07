package pages

import (
	"strconv"

	"tabletopper/internal/queries"
)

// THE ASSET MANAGER IS A PAGE PER KIND, and these four names are what says
// which one is being rendered. They live here rather than beside the markup in
// asset-tabs.templ because that file cannot carry a comment of any kind, and
// because the reason there are four of them is not obvious from a list of four
// strings.
//
// A kind is a page rather than a filter on one page because the four do not
// behave alike. A map is tiled by a background worker and its card polls; a
// token is one image with an aspect worth keeping; an avatar is a square
// thumbnail; music is not an image at all and never passes through the image
// routes. One page switching on a query parameter would be four pages sharing
// a URL.
//
// THE VALUES ARE THE LAST PATH SEGMENT of the page each one marks, which is
// what lets the controller pass the same word it routed on. They are not read
// from the URL -- each handler names its own -- so nothing here is a parameter
// anything could ask for.
const (
	assetTabMaps    = "maps"
	assetTabTokens  = "tokens"
	assetTabAvatars = "avatars"
	assetTabMusic   = "music"
)

// AssetNameLimit is what assets.name holds, and it is exported because the
// markup needs it: the name box on every asset card carries it as maxlength, so
// the browser refuses a value the column cannot take rather than the server
// discovering it at the insert. MySQL runs strict, so a longer value is a
// driver error rather than a truncation.
//
// It is counted in characters and not bytes, which is what VARCHAR counts and
// what maxlength counts -- so a name of 255 accented characters is legal in
// both places and is 400-odd bytes on the wire.
const AssetNameLimit = 255

// The asset manager's cards. A map is not one thing that is either there or
// not: it is an original in the bucket, a pyramid of tiles built from it by a
// background worker, and a job that may be queued, running, finished or given
// up on. The card has to say which of those is true without pretending they
// move together, because they do not.
//
// ALL OF THE REASONING ABOUT THIS CARD LIVES IN THIS FILE, and it has to. Its
// markup is assets.templ, which cannot carry a comment of any kind: Tailwind
// reads every .templ file as text and takes a class-name candidate from any
// word-shaped thing it finds, so a sentence of English containing "status" or
// "stack" emits a DaisyUI component family into the built stylesheet and
// nothing fails. The three questions below are methods rather than fields for
// the same reason -- `if m.Polling()` in the markup says what it is testing,
// where `if m.State == "working" || m.State == "pending"` would need a
// sentence next to it that there is nowhere to write.

// MapAsset is one map as its card needs it.
//
// Generation and State are the two halves that do not move together, and
// keeping them apart is the whole design. Generation is the pyramid that is
// serving right now; State is what the tiling job is doing. A map whose owner
// has just replaced it has a Generation from the previous upload and a State of
// pending, and it goes on being usable throughout -- the worker does not delete
// the old pyramid until it has a whole new one to put in its place.
//
// Everything else is a string because the controller does every conversion
// once and the template does none, which is what every page-data type here
// does.
type MapAsset struct {
	ID       string
	Name     string
	FileName string
	// Generation is the ULID of the pyramid that is serving, or empty when
	// nothing has ever finished being built for this map.
	Generation string
	// State is the tiling job's state, or the zero value for a row that has
	// never had one.
	State queries.AssetsTileState
}

// Usable answers whether this map can be put on a table: there is a complete
// pyramid in the bucket to draw it from. It says nothing about the job, and the
// job says nothing about it.
//
// IT IS ALSO WHAT THE PREVIEW HANGS ON, because the preview is built from the
// top of the pyramid and stored under the same generation. The worker writes
// preview_path and tile_gen in one statement -- see CompleteMapTiling in
// assets.sql -- so a map with a generation always has a thumbnail to show, and
// a map without one has no image to point an <img> at. Pointing at it anyway
// would not 404: the image route falls back to file_path, which is the
// original, which is a PNG or a JPEG served as image/webp.
func (m MapAsset) Usable() bool {
	return m.Generation != ""
}

// Polling answers whether the card should ask again in a moment: there is a
// tiling job queued or running, so what this card says is about to be wrong.
//
// A usable card polls too. That is the case the three questions exist to
// separate -- a replacement being built over a pyramid that is still serving --
// and a card that stopped showing the map while its replacement rendered would
// be worse than one that never updated.
func (m MapAsset) Polling() bool {
	return m.State == queries.AssetsTileStatePending || m.State == queries.AssetsTileStateWorking
}

// Retryable answers whether to offer the owner the button that queues the job
// again. Failed is a resting state: the worker retries a few times on its own
// and then stops, because a file that cannot be decoded will not decode on the
// fourth attempt either, and only a person can know that the thing to do now is
// try again anyway.
func (m MapAsset) Retryable() bool {
	return m.State == queries.AssetsTileStateFailed
}

// CardURL is where a polling card fetches its next self from. It is a method so
// that the route and the markup are joined in one place with the reason
// attached: this URL has to stay in step with the pattern registered in
// routes.go, and a card pointed at a path that no longer routes stops updating
// silently -- the swap never happens and the map simply never finishes.
func (m MapAsset) CardURL() string {
	return "/fragment/assets/maps/" + m.ID + "/card"
}

// nameBox and controls are what a map gives the two shared card components. See
// asset-card.go for why they are structs built here rather than arguments
// assembled in the markup.
//
// THE FIELD NAMES ARE THE MAP'S OWN and are not the library's. "map-name" and
// "map" are what UploadMap, ReplaceMap and ReplaceMapName read, and they were
// there before there was a second kind of asset to share a card with -- so they
// stay, and the components take the field name as a parameter rather than the
// handlers being rewritten to agree with a card.
func (m MapAsset) nameBox() nameBox {
	return nameBox{
		ID:        "map-name-" + m.ID,
		Value:     m.Name,
		URL:       "/assets/maps/" + m.ID + "/name",
		Field:     "map-name",
		MaxLength: strconv.Itoa(AssetNameLimit),
		Size:      nameBoxRoomy,
	}
}

func (m MapAsset) controls() cardControls {
	return cardControls{
		FileName:    m.FileName,
		ReplaceID:   "map-replace-" + m.ID,
		ReplaceURL:  "/assets/maps/" + m.ID,
		Field:       "map",
		Accept:      imageAccept,
		DeleteURL:   "/assets/maps/" + m.ID,
		ConfirmName: m.Name,
	}
}
