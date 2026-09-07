package pages

// THE GRID IS THE SAME SECTION ON ALL FOUR MANAGER PAGES, and what changes
// between them is the width of a card, what to say when there is nothing in it,
// and what to say when a search found nothing. This is that description.
//
// EVERY SENTENCE THE MANAGER SHOWS LIVES IN THIS FILE, and that is the point of
// it. The markup is assets.templ and its neighbours, which cannot carry a
// comment of any kind -- Tailwind reads every .templ file as text and takes a
// class-name candidate from anything word-shaped it finds, so a bare English
// word that happens to name a DaisyUI component emits that component's whole
// family into the built stylesheet and nothing fails. Prose is where those
// words turn up, because prose is where ordinary English gets written; the
// Avatars blurb below said "picking one at the table" for an afternoon and put
// fourteen `.table` rules in the build. Moving the prose out of the markup is
// the only fix that keeps working after somebody rewrites a sentence.
//
// THE CLASS NAMES CANNOT MOVE HERE WITH IT. app.css declares exactly one
// source, `@source "../templ/**/*.templ"`, so a class named only in a .go file
// is never emitted at all -- the column tracks below are named in assets.templ
// for that reason and referenced from here.

// assetGrid is one kind's grid as the section rendering it needs it.
type assetGrid struct {
	// Kind is the tab this page is, the id of the section, and the value the
	// search box sends as ?kind=. They are one string because they name one
	// thing: a box aimed at an id that is not on the page fails silently --
	// htmx logs to the console and the list simply never changes.
	Kind string

	// Cols is the responsive column track, which is what decides how big a card
	// is -- assetTileGrid, assetFaceGrid or assetRowGrid, all declared in
	// assets.templ because a class named only in a .go file is never built.
	//
	// EVERY ONE OF THEM IS auto-fill AND NOT A COLUMN COUNT, which is the whole
	// of the ultrawide fix. A fixed five columns is five columns at 3440 pixels
	// too, so each card came out 660 wide and every preview in it was a 256
	// pixel image blown up two and a half times. auto-fill sizes the CARD
	// instead and lets the count fall out of the width, so a card stays the size
	// its picture was stored at however wide the monitor is.
	//
	// THE MINIMUM IS THE STORED IMAGE PLUS THE CARD'S PADDING, which is why
	// there are two of them. A map preview is images.MapPreviewSize (256) and a
	// token is fitted into 512, so a 16rem track puts a card at 256 to 320 and
	// the picture is never upscaled. An avatar is a 256 pixel face and wants a
	// denser wall than that -- 11rem -- because the question the Avatars page
	// answers is "which of these forty", and forty faces at map size is
	// scrolling.
	Cols string

	// Query is the search that produced this grid, empty when it is the whole
	// shelf. IT IS HERE FOR THE EMPTY STATE AND NOTHING ELSE: a shelf with
	// nothing on it and a search that matched nothing are different things to
	// say to somebody, and the grid cannot tell them apart from its own length.
	Query string

	// EmptyHeading and EmptyBlurb are what an untouched page says. NoMatch is
	// the heading for a search that found nothing, assembled here rather than in
	// the markup so that not one word of it is in a scanned file; the line under
	// it is noMatchHint, which every list in the app shares.
	//
	// Both states render as the same noticePanel -- see notice-panel.go for why
	// they are one component.
	EmptyHeading string
	EmptyBlurb   string
	NoMatch      string
}

func mapsGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabMaps,
		Cols:         assetTileGrid,
		Query:        query,
		EmptyHeading: "No maps yet.",
		EmptyBlurb:   "A map is the board you play on. Upload one and it is cut into tiles, so it stays sharp however far in you zoom.",
		NoMatch:      noMatchHeading("maps", query),
	}
}

func tokensGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabTokens,
		Cols:         assetTileGrid,
		Query:        query,
		EmptyHeading: "No tokens yet.",
		EmptyBlurb:   "A token is a thing on the board that is not a creature -- a wagon, a rowboat, a barricade. They sit on their own layer, under the pawns.",
		NoMatch:      noMatchHeading("tokens", query),
	}
}

func avatarsGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabAvatars,
		Cols:         assetFaceGrid,
		Query:        query,
		EmptyHeading: "No avatars yet.",
		EmptyBlurb:   "A face to put on an NPC. Gather them here before a session, and spawning one mid-game is a search rather than a hunt through your folders.",
		NoMatch:      noMatchHeading("avatars", query),
	}
}

func musicGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabMusic,
		Cols:         assetRowGrid,
		Query:        query,
		EmptyHeading: "No music yet.",
		EmptyBlurb:   "The tracks you play behind a session -- a battle, a tavern, an hour of rain. One plays at a time and loops until you stop it or pick another.",
		NoMatch:      noMatchHeading("tracks", query),
	}
}

// assetSearchLabel is the search box's placeholder and its accessible name,
// which are the same string: the box has no visible label, so the placeholder
// is the only thing naming it on screen and aria-label is what names it to a
// screen reader. Two different words there would be one control with two names.
func assetSearchLabel(kind string) string {
	return "Search " + kind
}
