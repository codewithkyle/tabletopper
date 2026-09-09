package room

import "github.com/oklog/ulid/v2"

// THE SCENARIO: one session, played through, touching every command in both
// registries. It is written once and driven twice -- by the convergence test,
// which checks each step against the server's own state, and by the fixture
// generator, which writes it out for phase 3's TypeScript reducer to replay.
//
// IT IS WRITTEN IMPERATIVELY rather than as a table, because half the commands
// name something an earlier command created. A table would have to predict the
// ids, and predicting them would mean the test knew how many the commands in
// between happened to mint.
//
// IT READS LIKE A SESSION on purpose: the GM sets up a room, the party arrives,
// a fight happens, somebody is thrown out, and the table is packed away. A
// scenario built as a list of every command in registry order would exercise
// the same code and would never have caught anything that only goes wrong in
// sequence.
func scenario(r *recorder) {
	w := r.w
	gm, pc, other := w.gm, w.pc, w.other
	ground := w.layer

	// --- The GM sets the room up before anybody is looking. ---

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

	r.do("show the party exact hit points for a while", &TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, gm)

	// --- The party arrives. ---

	r.hub("lock the room once everybody is in", &RoomSetLocked{Locked: true})

	spawnAri := r.do("the GM puts Ari's character on the map", &PawnSpawn{
		Kind: PawnPlayer, Layer: ground, X: 300, Y: 300, Visible: true,
		CharacterID: &testCharID,
		Pawn: &Pawn{
			Name: "Ari", Image: "/assets/ari.webp", Size: SizeMedium,
			HP: intp(11), MaxHP: intp(14), AC: intp(16),
			OwnerID: &testPlayerID, CharacterID: &testCharID,
		},
	}, gm)
	ari := spawnAri[0].Event.(*PawnSpawned).Pawn.ID

	r.do("and Spawn pawns brings the rest of the party", &PawnSpawnCharacters{Pawns: []Pawn{{
		Name: "Rin", Image: "/assets/rin.webp", Size: SizeMedium, LayerID: ground,
		X: 380, Y: 300, Visible: true,
		HP: intp(9), MaxHP: intp(9), AC: intp(14),
		OwnerID: &testOtherID, CharacterID: &testOtherChar,
	}}}, gm)

	spawnGoblin := r.do("a goblin steps out", &PawnSpawn{
		Kind: PawnMonster, Layer: ground, X: 620, Y: 300, Visible: true,
		MonsterID: idp(testID(60)),
		Pawn: &Pawn{
			Name: "Goblin", Image: "/assets/goblin.webp", Size: SizeSmall,
			HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
		},
	}, gm)
	goblin := spawnGoblin[0].Event.(*PawnSpawned).Pawn.ID

	spawnAmbush := r.do("and one waits in the dark", &PawnSpawn{
		Kind: PawnMonster, Layer: ground, X: 900, Y: 300, Visible: false,
		MonsterID: idp(testID(60)),
		Pawn: &Pawn{
			Name: "Goblin", Image: "/assets/goblin.webp", Size: SizeSmall,
			HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
		},
	}, gm)
	ambusher := spawnAmbush[0].Event.(*PawnSpawned).Pawn.ID

	spawnWagon := r.do("the party's wagon is in the way", &PawnSpawn{
		Kind: PawnObject, Layer: ground, X: 480, Y: 480, Visible: true, Name: "Wagon",
		Pawn: &Pawn{
			Image: "/assets/wagon.webp", Width: 128, Height: 256,
			HP: intp(30), MaxHP: intp(30), AC: intp(12),
		},
	}, gm)
	wagon := spawnWagon[0].Event.(*PawnSpawned).Pawn.ID

	// AN OBJECT IS RESIZED AND TURNED AFTER IT IS DOWN, never before: the spawn
	// carries no size and no angle at all. The angle is written as -90 rather
	// than 270 because that is what a hand dragging the rotate handle
	// anticlockwise produces, and folding it is the server's job.
	r.do("turn the wagon across the road and stretch it", &PawnUpdate{
		ID: wagon, Width: intp(96), Height: intp(320), Rotation: intp(-90),
	}, gm)

	r.do("hide the numbers again now the fight is on", &TableSetOptions{PawnLabels: LabelsDefault, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, gm)

	// --- The fight. ---

	// SYNC BUILDS THE ORDER FROM THE TABLE and the GM drags it into shape
	// afterwards, which is the pair of gestures the feature is made of. Ari,
	// Rin and the goblin go in; the ambusher is hidden and the wagon is an
	// object, so neither does.
	r.do("the GM builds the turn order from the table", &InitiativeSync{}, gm)

	r.do("roll for initiative", &InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}, Initiative: 19},
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 14},
		{Name: "Lair action", Initiative: 20},
	}}, gm)

	r.do("Ari goes first", &InitiativeNext{}, gm)

	r.do("Ari moves up", &PawnMove{Anchor: ari, X: 430, Y: 300}, pc)
	r.do("and shows everybody where she is going next", &PawnDrag{Anchor: ari, X: 520, Y: 310}, pc)
	r.do("Ari ends her turn", &InitiativeNext{}, pc)

	r.do("the goblin takes a hit", &PawnUpdate{ID: goblin, HP: intp(3)}, gm)
	r.do("and goes prone", &PawnSetConditions{ID: goblin, Conditions: []Condition{
		{Name: "Prone", Color: ColorWhite, Duration: -1, Clear: ClearEnd},
		{Name: "Blessed", Color: ColorYellow, Duration: 2, Clear: ClearStart},
	}}, gm)
	r.do("the goblin shuffles back", &PawnMove{Anchor: goblin, X: 700, Y: 340}, gm)
	r.do("end of the goblin's turn", &InitiativeNext{}, gm)
	r.do("round two", &InitiativeNext{}, gm)

	r.do("the ambusher steps out", &PawnSetVisible{IDs: []ulid.ULID{ambusher}, Visible: true}, gm)
	r.do("no, back into the dark", &PawnSetVisible{IDs: []ulid.ULID{ambusher}, Visible: false}, gm)

	r.do("somebody points at the door", &Ping{Layer: ground, X: 512, Y: 96}, pc)

	// --- Fog and drawing. ---

	r.do("turn the fog on", &FogSetEnabled{Layer: ground, Enabled: true}, gm)
	r.do("this map is better uncovered", &FogSetPrefill{Layer: ground, Prefill: false}, gm)

	fogged := r.do("cover the far room", &FogAdd{
		Layer: ground, Kind: ShapeRect, Mode: FogHide, Points: []int{1024, 0, 2048, 1024},
	}, gm)
	hidden := fogged[0].Event.(*FogAdded).Shape.ID

	r.do("cut a corridor out of it", &FogAdd{
		Layer: ground, Kind: ShapePoly, Mode: FogReveal,
		Points: []int{1024, 400, 1400, 400, 1400, 560, 1024, 560},
	}, gm)

	r.do("that rectangle was in the wrong place", &FogRemove{ID: hidden}, gm)

	r.do("the GM sketches the plan", &StrokeBegin{
		ID: testID(900), Layer: ground, Color: "#ff0000ff", Width: 6, Points: []int{100, 100, 140, 130},
	}, gm)
	r.do("and keeps drawing", &StrokeExtend{ID: testID(900), Points: []int{180, 170, 220, 190}}, gm)
	r.do("and lifts the pen", &StrokeEnd{ID: testID(900)}, gm)

	r.do("Ari draws over it", &StrokeBegin{
		ID: testID(901), Layer: ground, Color: "#00ff00ff", Width: 3, Points: []int{300, 100, 320, 140},
	}, pc)
	r.do("and rubs her own line out", &StrokeErase{IDs: []ulid.ULID{testID(901)}}, pc)

	// --- Moving between floors. ---

	r.do("send the wagon down to the cellar", &PawnSetLayer{IDs: []ulid.ULID{wagon}, Layer: cellar}, gm)
	r.do("the party follows it down", &TableSetActiveLayer{Layer: cellar}, gm)
	r.do("and comes back up", &TableSetActiveLayer{Layer: ground}, gm)
	r.do("bring the wagon back too", &PawnSetLayer{IDs: []ulid.ULID{wagon}, Layer: ground}, gm)

	r.do("Ari asks for the whole room again", &SyncRequest{}, pc)
	r.do("and so does the GM", &SyncRequest{}, gm)

	// --- Packing away. ---

	r.hub("Rin drops off the wifi", &PlayerSetConnected{ID: testOtherID, Connected: false})
	r.hub("and comes back", &PlayerSetConnected{ID: testOtherID, Connected: true})
	r.hub("somebody new arrives", &PlayerJoin{Player: Player{
		ID: testID(1100), Name: "Wren", Role: RolePlayer,
	}})
	r.hub("and thinks better of it", &PlayerLeave{ID: testID(1100)})

	r.do("the goblin dies", &PawnRemove{IDs: []ulid.ULID{goblin}}, gm)
	r.do("clear the tracker", &InitiativeClear{}, gm)
	r.do("wipe the fog", &FogClear{Layer: ground}, gm)
	r.do("wipe the drawing", &StrokeClear{Layer: ground}, gm)

	r.do("Rin is thrown out", &PlayerKick{ID: testOtherID}, gm)
	_ = other

	r.do("the cellar is not needed after all", &TableRemoveLayer{Layer: cellar}, gm)
	r.do("clear the ground floor's map", &TableClearLayerMap{Layer: ground}, gm)

	// ONE COMMAND FOR WHAT THE FOUR ABOVE DID BY HAND. Everything still on the
	// table goes: the four remaining pawns, both floors' fog and drawing, the
	// maps, and the tracker. It is the end of the evening rather than a tool
	// used during one, which is why it is here and not up in the fight.
	r.do("and clear the tabletop for next week", &TableClear{}, gm)

	r.hub("unlock on the way out", &RoomSetLocked{Locked: false})
	r.hub("and close the room", &RoomClose{})
}
