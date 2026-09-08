package pages

import (
	"strconv"

	"tabletopper/internal/queries"
)

// THE MAP PICKER, which is a content modal and not a window.
//
// Choosing a map is one act with an end: it is opened from a row in the layer
// manager, it answers one question, and it closes. That is the whole of the
// difference between the two mechanisms, and it is why this is the only thing
// in the table family that sends HX-Trigger modal:close.
//
// IT SHOWS EVERY MAP THE GM OWNS, TILED OR NOT, and only the tiled ones are
// buttons. That is a change from what it did first, which was to list the ready
// ones and tell anybody with nothing ready to go to the Maps page -- a sentence
// that ended the session's train of thought and reappeared on every fresh
// account. A map is uploaded from in here now, so the picker has to be able to
// show a map that does not exist yet as far as the bucket is concerned: the card
// says it is building, asks again every couple of seconds, and turns into a
// button of its own accord when the tiles land. Nothing else about choosing
// changed -- hub.resolveMap still refuses a map that is not ready, so the card
// not being a button is the courtesy and not the rule.
//
// THE CARD IS THE UNIT, exactly as it is on the asset manager. An upload is
// appended to the grid the moment the row exists and polls itself from there;
// nothing refetches the list, so a card finishing cannot move the cards around
// it or throw away the scroll position of somebody reading them. It is the
// pattern MapCard has used since there was a tiling worker, and the reason both
// of them stop polling by themselves: the poll lives in the card's own markup,
// so a card that comes back finished comes back without an hx-trigger.
//
// THE SIZE IS LOCKED RATHER THAN FITTED TO THE CONTENTS. A dialog that is as
// tall as its list is a dialog that resizes under the cursor on every keystroke
// in the search box, and the button somebody was aiming at moves. It is one
// width and one height whatever is in it, which is also why the grid scrolls
// rather than the modal growing.
//
// THERE IS NO WAY OUT OF HERE TO THE ASSET MANAGER, deliberately. The button
// that used to be there was a link off the room page in the middle of a session,
// and everything it was reached for -- upload a map, find a map, try a failed one
// again -- is in this dialog now.

// RoomMapListID is the id of the grid, which three things have to agree on: the
// grid names itself with it, the search box aims at it, and the upload prepends
// through it. A target that has drifted from its element fails silently -- htmx
// logs to the console and nothing lands.
const RoomMapListID = "room-map-list"

// RoomMapsData is the picker.
type RoomMapsData struct {
	RoomID  string
	LayerID string

	// Query is the search that produced this grid, empty when it is the whole
	// library. IT IS HERE FOR THE EMPTY STATE AND NOTHING ELSE: a library with
	// no maps in it and a search that matched nothing are different things to
	// say to somebody, and the grid cannot tell them apart from its own length.
	Query string

	Maps []RoomMapChoice
}

// RoomMapChoice is one card, and it carries the room and the layer because it
// has to stand on its own: an upload appends one of these into a grid, and a
// card that is still building fetches its own replacement. Neither of those has
// a RoomMapsData anywhere near it.
//
// Generation and State are the two halves of what a map's tiling is doing and
// they do not move together -- see MapAsset in assets.go, which is the same
// distinction on the asset manager's card and carries the whole reasoning.
type RoomMapChoice struct {
	RoomID     string
	LayerID    string
	ID         string
	Name       string
	FileName   string
	Width      int
	Height     int
	Generation string
	State      queries.AssetsTileState
}

// ListPath is the grid on its own, which is what a search replaces.
func (d RoomMapsData) ListPath() string {
	return "/fragment/room/map-list?room=" + d.RoomID + "&layer=" + d.LayerID
}

// UploadPath is the picker's upload.
//
// IT IS A ROOM ROUTE AND NOT /assets/maps, and the difference is the card that
// comes back rather than the work that is done. Both go through storeMap and
// both write the GM's own asset -- a map uploaded from a room appears on the
// Maps page and is theirs at the next table too -- but the manager's card is a
// name box with a Replace and a Delete on it, and this one is a button that puts
// the map on a layer. The layer is in the path because the card that comes back
// posts to it.
func (d RoomMapsData) UploadPath() string {
	return "/rooms/" + d.RoomID + "/layers/" + d.LayerID + "/maps"
}

// SetPath is the layer's map resource, which a card posts to with its own asset
// id. It is the layer's URL and not the picker's, because what is being changed
// is the layer.
func (m RoomMapChoice) SetPath() string {
	return "/rooms/" + m.RoomID + "/layers/" + m.LayerID + "/map"
}

// CardURL is where a building card fetches its next self from. It is a method so
// that the route and the markup are joined in one place with the reason
// attached: a card pointed at a path that no longer routes stops updating
// silently -- the swap never happens and the map simply never finishes.
func (m RoomMapChoice) CardURL() string {
	return "/fragment/room/map-card?room=" + m.RoomID + "&layer=" + m.LayerID + "&asset=" + m.ID
}

// RetryPath queues a failed map's tiling again and answers with this card. It is
// offered here because there is no longer a link out of this dialog: a map that
// gave up while the GM was looking at it would otherwise be a card that says so
// and nothing to do about it.
func (m RoomMapChoice) RetryPath() string {
	return "/rooms/" + m.RoomID + "/layers/" + m.LayerID + "/maps/" + m.ID
}

func (m RoomMapChoice) PreviewURL() string {
	return "/assets/images/" + m.ID + "/preview"
}

// Usable answers whether this map can go on a layer: there is a complete pyramid
// in the bucket to draw it from. It is also what the preview hangs on, because
// the preview is written under the same generation as the tiles -- so a map
// without one has no thumbnail to point an <img> at.
func (m RoomMapChoice) Usable() bool {
	return m.Generation != ""
}

// Polling answers whether this card is about to be wrong. A usable map polls too
// -- that is a replacement being built over a pyramid that is still serving --
// and it stays choosable throughout.
func (m RoomMapChoice) Polling() bool {
	return m.State == queries.AssetsTileStatePending || m.State == queries.AssetsTileStateWorking
}

// Retryable answers whether to offer the button that queues the job again.
// Failed is a resting state: the worker tries a few times on its own and then
// stops, because a file that cannot be decoded will not decode on the fourth
// attempt either, and only a person can know that the thing to do now is try
// again anyway.
func (m RoomMapChoice) Retryable() bool {
	return m.State == queries.AssetsTileStateFailed
}

// CardClass is the card's border, which says whether the card is a button.
//
// THE TWO STRINGS IT CHOOSES BETWEEN ARE IN room-maps.templ, and they are whole
// class lists rather than a base plus a suffix. app.css declares exactly one
// source, `@source "../templ/**/*.templ"`, so a class named only in a .go file
// is never emitted at all -- and a name assembled from fragments is not a name
// Tailwind can see either. asset-grid.go names the manager's column tracks the
// same way and for the same reason.
func (m RoomMapChoice) CardClass() string {
	if m.Usable() {
		return roomMapPickableBox
	}

	return roomMapCardBox
}

// Size is the map's dimensions, which is the one fact that decides whether a map
// belongs on the same table as the others: the grid is room-wide, so two floors
// exported at different scales will not line up.
//
// IT IS EMPTY UNTIL THE MAP HAS BEEN TILED, because the width and height are
// written by the worker that tiles it -- they come from the decoded image, and
// nothing has decoded it yet. The card leaves the line out rather than printing
// the zeroes.
func (m RoomMapChoice) Size() string {
	if m.Width < 1 || m.Height < 1 {
		return ""
	}

	return strconv.Itoa(m.Width) + " × " + strconv.Itoa(m.Height)
}

// FileLabel is the file the map came from, shown only when the GM has renamed
// it. Unchanged it is the same string as the name -- storeMap fills both from the
// filename -- and printing it twice on one card would look like a bug.
//
// IT IS ON THE CARD BECAUSE THE SEARCH MATCHES IT. SearchPickerMaps looks at both
// names, so a term that hit nothing visible would otherwise be a card that
// appears to have matched nothing in it.
func (m RoomMapChoice) FileLabel() string {
	if m.FileName == m.Name {
		return ""
	}

	return m.FileName
}

// The picker's own empty states, here rather than in the markup because Tailwind
// reads every .templ file as text and takes a class-name candidate from anything
// word-shaped it finds -- so a bare English word that happens to name a DaisyUI
// component emits that component's whole family into the built stylesheet, and
// nothing fails. See asset-grid.go, where the manager's four sets of the same
// sentences live for the same reason.
const (
	roomMapsEmptyHeading = "No maps yet."
	roomMapsEmptyBlurb   = "Upload one and it is cut into tiles, so it stays sharp however far in you zoom. It becomes choosable here the moment they are ready."
)

func (d RoomMapsData) NoMatchHeading() string { return noMatchHeading("maps", d.Query) }

func (d RoomMapsData) EmptyHeading() string { return roomMapsEmptyHeading }

func (d RoomMapsData) EmptyBlurb() string { return roomMapsEmptyBlurb }

// SearchLabel is the box's placeholder and its accessible name, which are one
// string: the box has no visible label, so the placeholder is the only thing
// naming it on screen and aria-label is what names it to a screen reader.
func (d RoomMapsData) SearchLabel() string { return "Search your maps" }

// NameLimit is the search box's maxlength, which is the column's width. A term
// longer than a name can be is a term nothing can match, and the fragment
// answers one with a 404 -- so the box refuses it before the request leaves.
func (d RoomMapsData) NameLimit() string { return strconv.Itoa(AssetNameLimit) }

// UploadAccept is the same filter every image upload in the app carries.
func (d RoomMapsData) UploadAccept() string { return imageAccept }

// SearchTriggers is what the box fetches on: typing, and the little clear button
// a search input draws.
func (d RoomMapsData) SearchTriggers() string { return "input changed delay:250ms, search" }
