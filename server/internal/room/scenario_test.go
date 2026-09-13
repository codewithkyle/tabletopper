package room

import "github.com/oklog/ulid/v2"

func scenario(r *recorder) {
	w := r.w
	gm, pc, other := w.gm, w.pc, w.other
	ground := w.layer
	r.hub("name the room", &RoomSetName{Name: "The Sunless Citadel"})
	r.do("put a map on the ground floor", &TableSetLayerMap{
		Layer:   ground,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, gm)
	r.do("add a cellar", &TableAddLayer{Name: "Cellar"}, gm)
	cellar := w.s.Table.Layers[len(w.s.Table.Layers)-1].ID
	r.do("rename it now that it is flooded", &TableRenameLayer{Layer: cellar, Name: "Cellar, flooded"}, gm)
	r.do("map the cellar too", &TableSetLayerMap{
		Layer:   cellar,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(51), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2},
	}, gm)
	r.do("move the cellar under the ground floor", &TableMoveLayer{Layer: cellar, Index: 0}, gm)
	r.do("and back on top of it", &TableMoveLayer{Layer: cellar, Index: 1}, gm)
	grid := w.s.Table.Grid
	grid.CellSize = 70
	grid.OffsetX, grid.OffsetY = -12, 8
	grid.Color = "#334455ff"
	r.do("line the grid up with the map", &TableSetGrid{Grid: grid}, gm)
	r.do("mark where the party comes in", &TableSetPartyStart{Layer: ground, X: intp(640), Y: intp(320)}, gm)
	r.do("show the party exact hit points for a while", &TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, gm)
	r.hub("lock the room once everybody is in", &RoomSetLocked{Locked: true})
	r.do("the GM puts Ari's character on the map", &PawnSpawn{
		Kind: PawnPlayer, Layer: ground, X: 300, Y: 300, Visible: true,
		CharacterID: &testCharID,
		Pawn: &Pawn{
			Name: "Ari", Image: "/assets/ari.webp", Size: SizeMedium,
			HP: intp(11), MaxHP: intp(14), AC: intp(16),
			OwnerID: &testPlayerID, CharacterID: &testCharID,
		},
	}, gm)
	ari := newestPawn(w.s)
	r.do("and Spawn pawns brings the rest of the party", &PawnSpawnCharacters{Pawns: []Pawn{{
		Name: "Rin", Image: "/assets/rin.webp", Size: SizeMedium, LayerID: ground,
		X: 380, Y: 300, Visible: true,
		HP: intp(9), MaxHP: intp(9), AC: intp(14),
		OwnerID: &testOtherID, CharacterID: &testOtherChar,
	}}}, gm)
	r.do("a goblin steps out", &PawnSpawn{
		Kind: PawnMonster, Layer: ground, X: 620, Y: 300, Visible: true,
		MonsterID: idp(testID(60)),
		Pawn: &Pawn{
			Name: "Goblin", Image: "/assets/goblin.webp", Size: SizeSmall,
			HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
		},
	}, gm)
	goblin := newestPawn(w.s)
	r.do("and one waits in the dark", &PawnSpawn{
		Kind: PawnMonster, Layer: ground, X: 900, Y: 300, Visible: false,
		MonsterID: idp(testID(60)),
		Pawn: &Pawn{
			Name: "Goblin", Image: "/assets/goblin.webp", Size: SizeSmall,
			HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
		},
	}, gm)
	ambusher := newestPawn(w.s)
	r.do("the party's wagon is in the way", &PawnSpawn{
		Kind: PawnObject, Layer: ground, X: 480, Y: 480, Visible: true, Name: "Wagon",
		Pawn: &Pawn{
			Image: "/assets/wagon.webp", Width: 128, Height: 256,
			HP: intp(30), MaxHP: intp(30), AC: intp(12),
		},
	}, gm)
	wagon := newestPawn(w.s)
	r.do("turn the wagon across the road and stretch it", &PawnUpdate{
		ID: wagon, Width: intp(96), Height: intp(320), Rotation: intp(-90),
	}, gm)
	r.do("hide the numbers again now the fight is on", &TableSetOptions{PawnLabels: LabelsDefault, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, gm)
	r.do("the GM builds the turn order from the table", &InitiativeSync{}, gm)
	r.do("roll for initiative", &InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}, Initiative: 19},
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 14},
		{Name: "Lair action", Initiative: 20},
	}}, gm)
	r.do("and the GM rolls the order", &InitiativeRoll{Bonuses: map[ulid.ULID]int{
		entryNamed(w.s, "Ari"):    3,
		entryNamed(w.s, "Goblin"): 2,
	}}, gm)
	r.do("Ari goes first", &InitiativeNext{}, gm)
	r.do("Ari moves up", &PawnMove{Anchor: ari, X: 430, Y: 300}, pc)
	r.do("and shows everybody where she is going next", &PawnDrag{Anchor: ari, X: 520, Y: 310}, pc)
	r.do("Ari swings at the goblin", &DiceRoll{Expr: "1d20 + 7", Label: "Longsword"}, pc)
	r.do("the flanking gives her advantage", &DiceRoll{Expr: "1d20 + 7", Label: "Longsword", Adv: AdvHigh}, pc)
	r.do("and she rolls the damage", &DiceRoll{Expr: "1d8 + 4", Label: "Longsword damage"}, pc)
	r.do("the GM rolls the goblin's save behind the screen", &DiceRoll{Expr: "1d20 - 1", Secret: true}, gm)
	r.do("and Ari checks something of her own the same way", &DiceRoll{Expr: "1d20 + 2", Label: "Stealth", Secret: true}, pc)
	r.do("the GM rolls the next one in the open", &DiceRoll{Expr: "1d20 - 1"}, gm)
	r.do("the GM puts a tavern song on", &MusicLoad{AssetID: testTrackID, Track: &TrackInfo{Name: "Tavern Brawl"}}, gm)
	r.do("and sets it to repeat", &MusicSetLoop{Loop: true}, gm)
	r.do("somebody is talking, so it pauses", &MusicPause{}, gm)
	r.do("and starts again", &MusicPlay{}, gm)
	r.do("the fight starts, so it stops", &MusicStop{}, gm)
	r.do("one last run through", &MusicPlay{}, gm)
	r.do("this time it plays out", &MusicSetLoop{Loop: false}, gm)
	r.do("and it reaches the end", &MusicEnded{TrackID: testTrackID}, gm)
	r.do("Ari ends her turn", &InitiativeNext{}, pc)
	r.do("the goblin takes a hit", &PawnUpdate{ID: goblin, HP: intp(3)}, gm)
	r.do("and goes prone", &PawnSetConditions{ID: goblin, Conditions: []Condition{
		{Name: "Prone", Color: ColorWhite, Duration: -1, Clear: ClearEnd},
		{Name: "Blessed", Color: ColorYellow, Duration: 2, Clear: ClearStart},
	}}, gm)
	r.do("the goblin shuffles back", &PawnMove{Anchor: goblin, X: 700, Y: 340}, gm)
	r.do("end of the goblin's turn", &InitiativeNext{}, gm)
	r.do("round two", &InitiativeNext{}, gm)
	r.do("a lair action is added by name", &InitiativeAdd{Name: "The volcano erupts"}, gm)
	r.do("and the ambusher from its own menu", &InitiativeAdd{Pawn: &ambusher}, gm)
	upsideDown := make([]ulid.ULID, 0, len(w.s.Initiative.Entries))
	for i := len(w.s.Initiative.Entries) - 1; i >= 0; i-- {
		upsideDown = append(upsideDown, w.s.Initiative.Entries[i].ID)
	}
	r.do("the GM drags the order upside down", &InitiativeReorder{IDs: upsideDown}, gm)
	r.do("and gives the turn to the volcano", &InitiativeActivate{Entry: entryNamed(w.s, "The volcano erupts")}, gm)
	r.do("then takes it out of the fight", &InitiativeRemove{Entry: entryNamed(w.s, "The volcano erupts")}, gm)
	r.do("the ambusher steps out", &PawnSetVisible{IDs: []ulid.ULID{ambusher}, Visible: true}, gm)
	r.do("no, back into the dark", &PawnSetVisible{IDs: []ulid.ULID{ambusher}, Visible: false}, gm)
	r.do("somebody points at the door", &Ping{Layer: ground, X: 512, Y: 96}, pc)
	r.do("turn the fog on", &FogSetEnabled{Layer: ground, Enabled: true}, gm)
	r.do("this map is better uncovered", &FogSetPrefill{Layer: ground, Prefill: false}, gm)
	r.do("cover the far room", &FogAdd{
		Layer: ground, Kind: ShapeRect, Mode: FogHide, Points: []int{1024, 0, 2048, 1024},
	}, gm)
	hidden := w.s.Fog[len(w.s.Fog)-1].ID
	r.do("cut a corridor out of it", &FogAdd{
		Layer: ground, Kind: ShapePoly, Mode: FogReveal,
		Points: []int{1024, 400, 1400, 400, 1400, 560, 1024, 560},
	}, gm)
	r.do("that rectangle was in the wrong place", &FogRemove{ID: hidden}, gm)
	r.do("the GM sketches the plan", &StrokeBegin{
		ID: testID(900), Layer: ground, Kind: StrokeFree, Color: "#ff0000ff", Width: 6, Points: []int{100, 100, 140, 130},
	}, gm)
	r.do("and keeps drawing", &StrokeExtend{ID: testID(900), Points: []int{180, 170, 220, 190}}, gm)
	r.do("and lifts the pen", &StrokeEnd{ID: testID(900)}, gm)
	r.do("Ari draws over it", &StrokeBegin{
		ID: testID(901), Layer: ground, Kind: StrokeFree, Color: "#00ff00ff", Width: 3, Points: []int{300, 100, 320, 140},
	}, pc)
	r.do("and rubs her own line out", &StrokeErase{IDs: []ulid.ULID{testID(901)}}, pc)
	r.do("send the wagon down to the cellar", &PawnSetLayer{IDs: []ulid.ULID{wagon}, Layer: cellar}, gm)
	r.do("the party follows it down", &TableSetActiveLayer{Layer: cellar}, gm)
	r.do("and comes back up", &TableSetActiveLayer{Layer: ground}, gm)
	r.do("bring the wagon back too", &PawnSetLayer{IDs: []ulid.ULID{wagon}, Layer: ground}, gm)
	r.do("Ari asks for the whole room again", &SyncRequest{}, pc)
	r.do("and so does the GM", &SyncRequest{}, gm)
	r.hub("Rin drops off the wifi", &PlayerSetConnected{ID: testOtherID, Connected: false})
	r.hub("and comes back", &PlayerSetConnected{ID: testOtherID, Connected: true})
	r.hub("somebody new arrives", &PlayerJoin{Player: Player{
		ID: testID(1100), Name: "Wren", Role: RolePlayer,
	}})
	r.hub("and thinks better of it", &PlayerLeave{ID: testID(1100)})
	r.hub("Ari patches her sheet between rounds", &CharacterSync{Info: CharacterInfo{
		ID: testCharID, OwnerID: testPlayerID, Name: "Ari Duskhollow",
		Size: SizeSmall, HP: 6, MaxHP: 16, AC: 17, Image: "/assets/ari.webp",
	}})
	r.do("the GM edits the goblin in one form", &Batch{Commands: []Command{
		&PawnUpdate{ID: goblin, Name: strp("Goblin boss"), AC: intp(13)},
		&PawnSetConditions{ID: goblin, Conditions: []Condition{
			{Name: "Frightened", Color: ColorBlue, Duration: 1, Clear: ClearStart},
		}},
	}}, gm)
	r.do("the goblin dies", &PawnRemove{IDs: []ulid.ULID{goblin}}, gm)
	r.do("clear the tracker", &InitiativeClear{}, gm)
	r.do("wipe the fog", &FogClear{Layer: ground}, gm)
	r.do("wipe the drawing", &StrokeClear{Layer: ground}, gm)
	r.do("Rin is thrown out", &PlayerKick{ID: testOtherID}, gm)
	_ = other
	r.do("the cellar is not needed after all", &TableRemoveLayer{Layer: cellar}, gm)
	r.do("clear the ground floor's map", &TableClearLayerMap{Layer: ground}, gm)
	r.do("and clear the tabletop for next week", &TableClear{}, gm)
	r.hub("open next week's prepped ambush over the empty table", &SceneLoad{Scene: preppedScene()})
	r.hub("unlock on the way out", &RoomSetLocked{Locked: false})
	r.hub("and close the room", &RoomClose{})
}
func preppedScene() *State {
	floor := testID(1200)
	s := &State{
		Schema: Schema,
		Table: Table{
			Layers: []Layer{{
				ID:         floor,
				Name:       "Ambush point",
				FogEnabled: true,
				FogPrefill: true,
				Map:        &MapRef{AssetID: testAssetID, Gen: testID(52), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2},
			}},
			TableSettings: TableSettings{
				ActiveLayer: floor,
				Grid: Grid{
					Lines:       GridLinesDashed,
					CellSize:    80,
					Color:       DefaultGridColor,
					Snap:        SnapHalfCells,
					FeetPerCell: 5,
					Diagonals:   DiagonalsEqual,
				},
			},
		},
		Pawns: []Pawn{
			{
				ID: testID(1201), Kind: PawnMonster, LayerID: floor, Name: "Goblin",
				Image: "/assets/goblin.webp", X: 640, Y: 320, Size: SizeSmall, Visible: true,
				HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
			},
			{
				ID: testID(1202), Kind: PawnObject, LayerID: floor, Name: "Cart",
				Image: "/assets/wagon.webp", X: 480, Y: 480, Width: 128, Height: 256, Visible: true,
			},
		},
		Fog: []FogShape{{
			ID: testID(1203), LayerID: floor, Kind: ShapeRect, Mode: FogHide,
			Points: []int{0, 0, 1024, 1024},
		}},
		Strokes: []Stroke{{
			ID: testID(1204), LayerID: floor, Kind: StrokeFree, Color: "#ff0000ff",
			Width: 4, Points: []int{16, 16, 64, 64}, Done: true,
		}},
	}
	s.Normalize()
	return s
}
func entryNamed(s *State, name string) ulid.ULID {
	for _, e := range s.Initiative.Entries {
		if e.Name == name {
			return e.ID
		}
	}
	return ulid.ULID{}
}
