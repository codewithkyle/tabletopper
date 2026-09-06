package pages

import "tabletopper/internal/queries"

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
