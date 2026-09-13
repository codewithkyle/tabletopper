package room

import "testing"

func TestThePartyStartsWhereTheGMMarkedIt(t *testing.T) {
	w := newWorld(t)
	w.apply(&TableSetLayerMap{
		Layer:   w.layer,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)
	lib := newLibrary()
	lib.characters[testCharID] = CharacterInfo{ID: testCharID, OwnerID: testPlayerID, Name: "Ari", Size: SizeMedium, HP: 11, MaxHP: 14, AC: 16}
	lib.characters[testOtherChar] = CharacterInfo{ID: testOtherChar, OwnerID: testOtherID, Name: "Rin", Size: SizeMedium, HP: 9, MaxHP: 9, AC: 14}
	w.apply(&PlayerSetConnected{ID: testPlayerID, Connected: true}, w.gm)
	w.apply(&PlayerSetConnected{ID: testOtherID, Connected: true}, w.gm)

	middle := &PawnSpawnCharacters{}
	w.resolve(middle, lib)
	if middle.Pawns[0].Y != 2048 {
		t.Fatalf("with no mark the party lands at y=%d, want the map's centre 2048", middle.Pawns[0].Y)
	}

	w.apply(&TableSetPartyStart{Layer: w.layer, X: intp(640), Y: intp(320)}, w.gm)
	marked := &PawnSpawnCharacters{}
	w.resolve(marked, lib)
	for _, p := range marked.Pawns {
		if p.Y != 320 {
			t.Errorf("%s lands at y=%d, want the marked 320", p.Name, p.Y)
		}
	}
	if len(marked.Pawns) < 2 {
		t.Fatalf("only %d pawns were placed", len(marked.Pawns))
	}
	if marked.Pawns[0].X == marked.Pawns[1].X {
		t.Error("the party is stacked on one square rather than spread across the mark")
	}
}
func TestTheMarkIsTheGMsAndComesOffAgain(t *testing.T) {
	w := newWorld(t)
	w.apply(&TableSetPartyStart{Layer: w.layer, X: intp(64), Y: intp(64)}, w.gm)
	if got := layerNamed(t, w.s, w.layer).PartyStart; got == nil || *got != (Point{X: 64, Y: 64}) {
		t.Fatalf("the mark is %+v", got)
	}
	w.apply(&TableSetPartyStart{Layer: w.layer}, w.gm)
	if got := layerNamed(t, w.s, w.layer).PartyStart; got != nil {
		t.Fatalf("a mark with no coordinates left %+v behind", got)
	}
	w.refuse(&TableSetPartyStart{Layer: w.layer, X: intp(1), Y: intp(1)}, w.pc, CodeForbidden)
	w.refuse(&TableSetPartyStart{Layer: testID(9999), X: intp(1), Y: intp(1)}, w.gm, CodeNotFound)
	w.refuse(&TableSetPartyStart{Layer: w.layer, X: intp(CoordLimit + 1), Y: intp(1)}, w.gm, CodeInvalid)
}
func TestTheMarkTravelsWithTheSceneAndNotWithThePeople(t *testing.T) {
	w := sceneWorld(t)
	w.apply(&TableSetPartyStart{Layer: w.layer, X: intp(640), Y: intp(320)}, w.gm)
	body, err := w.s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	mark := floorCalled(scene, DefaultLayerName).PartyStart
	if mark == nil || *mark != (Point{X: 640, Y: 320}) {
		t.Fatalf("the scene forgot where the party starts: %+v", mark)
	}
	live := liveWorld(t)
	live.s.ImportScene(scene)
	if got := floorCalled(live.s, DefaultLayerName).PartyStart; got == nil || *got != (Point{X: 640, Y: 320}) {
		t.Fatalf("the mark did not cross into the room: %+v", got)
	}
	*mark = Point{X: 1, Y: 1}
	if got := floorCalled(live.s, DefaultLayerName).PartyStart; *got != (Point{X: 640, Y: 320}) {
		t.Fatal("the room and the scene share one mark, so editing either edits both")
	}
}
