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
		EmptyHeading: "No maps yet",
		EmptyBlurb:   "A map is the board you play on.",
		NoMatch:      noMatchHeading("maps", query),
	}
}
func terrainGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabTerrain,
		Cols:         assetTileGrid,
		Query:        query,
		EmptyHeading: "No terrain yet",
		EmptyBlurb:   "The pictures you stamp onto a hex crawl (forest, hills, a river bend).",
		NoMatch:      noMatchFor("terrain", "matches", query),
	}
}
func tokensGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabTokens,
		Cols:         assetTileGrid,
		Query:        query,
		EmptyHeading: "No tokens yet",
		EmptyBlurb:   "A token is a thing on the board that you can move or resize (a wagon, rowboat, barricade).",
		NoMatch:      noMatchHeading("tokens", query),
	}
}
func avatarsGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabAvatars,
		Cols:         assetFaceGrid,
		Query:        query,
		EmptyHeading: "No avatars yet",
		EmptyBlurb:   "A face to put on an NPC.",
		NoMatch:      noMatchHeading("avatars", query),
	}
}
func musicGrid(query string) assetGrid {
	return assetGrid{
		Kind:         assetTabMusic,
		Cols:         assetRowGrid,
		Query:        query,
		EmptyHeading: "No music yet",
		EmptyBlurb:   "The tracks you play during a session.",
		NoMatch:      noMatchHeading("tracks", query),
	}
}
func assetSearchLabel(kind string) string {
	return "Search " + kind
}
