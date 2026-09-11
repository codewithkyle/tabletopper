package pages

type assetGrid struct {
	Kind         string
	Cols         string
	Query        string
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
func assetSearchLabel(kind string) string {
	return "Search " + kind
}
