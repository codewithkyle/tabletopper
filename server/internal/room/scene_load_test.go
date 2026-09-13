package room

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func savedScene(t *testing.T) *State {
	t.Helper()
	body, err := sceneWorld(t).s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return scene
}
func stockedLibrary(gen ulid.ULID) *fakeLibrary {
	lib := newLibrary()
	lib.maps[testAssetID] = MapRef{AssetID: testAssetID, Gen: gen, Width: 8192, Height: 8192, TileSize: 512, MaxZoom: 4}
	lib.maps[sceneCellarAsset] = MapRef{AssetID: sceneCellarAsset, Gen: gen, Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2}
	return lib
}
func floorCalled(s *State, name string) *Layer {
	for i := range s.Table.Layers {
		if s.Table.Layers[i].Name == name {
			return &s.Table.Layers[i]
		}
	}
	return nil
}
func TestARetiledMapIsReadAgainRatherThanTrusted(t *testing.T) {
	scene := savedScene(t)
	stale := scene.Table.Layers[0].Map.Gen
	fresh := testID(77)
	cmd := &SceneLoad{Scene: scene}
	if err := cmd.Resolve(context.Background(), stockedLibrary(fresh), liveWorld(t).s); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cmd.Missing) != 0 {
		t.Fatalf("a scene whose maps are all there reports %v", cmd.Missing)
	}
	for _, l := range cmd.Scene.Table.Layers {
		if l.Map == nil {
			t.Fatalf("%q lost its map", l.Name)
		}
		if l.Map.Gen == stale {
			t.Errorf("%q kept the generation it was saved with; its tiles are gone", l.Name)
		}
		if l.Map.Gen != fresh {
			t.Errorf("%q is on generation %s, want %s", l.Name, l.Map.Gen, fresh)
		}
	}
	if cmd.Scene.Table.Layers[0].Map.Width != 8192 {
		t.Errorf("the re-read map kept the old size %d", cmd.Scene.Table.Layers[0].Map.Width)
	}
}
func TestAMapThatIsGoneClearsItsFloorAndSaysWhich(t *testing.T) {
	scene := savedScene(t)
	lib := newLibrary()
	lib.maps[testAssetID] = MapRef{AssetID: testAssetID, Gen: testID(77), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3}
	cmd := &SceneLoad{Scene: scene}
	if err := cmd.Resolve(context.Background(), lib, liveWorld(t).s); err != nil {
		t.Fatalf("a scene with one missing map failed the whole load: %v", err)
	}
	if len(cmd.Missing) != 1 {
		t.Fatalf("Missing = %v, want one floor", cmd.Missing)
	}
	if !strings.Contains(cmd.Missing[0], "Cellar") {
		t.Errorf("the report does not name the floor: %q", cmd.Missing[0])
	}
	if !strings.Contains(cmd.Missing[0], "no longer in your library") {
		t.Errorf("the report does not carry the library's own message: %q", cmd.Missing[0])
	}
	if l := floorCalled(cmd.Scene, "Cellar"); l == nil || l.Map != nil {
		t.Error("the floor whose map is gone still points at it, so it renders an empty grid and says nothing")
	}
	if l := floorCalled(cmd.Scene, DefaultLayerName); l == nil || l.Map == nil {
		t.Error("the floor whose map is fine lost it too")
	}
}
func TestALibraryThatIsBrokenFailsTheWholeLoad(t *testing.T) {
	scene := savedScene(t)
	lib := newLibrary()
	lib.broken = errors.New("the database is down")
	cmd := &SceneLoad{Scene: scene}
	err := cmd.Resolve(context.Background(), lib, liveWorld(t).s)
	if err == nil {
		t.Fatal("a broken library loaded a scene with every map silently cleared")
	}
	if _, isRoomError := err.(*Error); isRoomError {
		t.Fatalf("a database failure was turned into a refusal the GM sees: %v", err)
	}
}
func TestLoadingPutsTheSceneOnTheTableAndLeavesThePeople(t *testing.T) {
	scene := savedScene(t)
	live := liveWorld(t)
	players := mustJSON(t, live.s.Players)
	cmd := &SceneLoad{Scene: scene}
	if err := cmd.Resolve(context.Background(), stockedLibrary(testID(77)), live.s); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := cmd.Apply(live.s, live.gm, live.env); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, name := range []string{"Goblin", "Wagon"} {
		if !hasPawnNamed(live.s, name) {
			t.Errorf("the %s did not arrive", name)
		}
	}
	for _, p := range live.s.Pawns {
		if p.Kind == PawnPlayer || p.CharacterID != nil {
			t.Errorf("%q is somebody's pawn and it is on the table after a load", p.Name)
		}
	}
	if len(live.s.Initiative.Entries) != 0 {
		t.Errorf("the turn order survived the load: %+v", live.s.Initiative)
	}
	if got := mustJSON(t, live.s.Players); got != players {
		t.Errorf("the people at the table changed:\n got %s\nwant %s", got, players)
	}
}
func TestASceneLoadIsTheGMsAndRefusesAnEmptyScene(t *testing.T) {
	live := liveWorld(t)
	bare := &SceneLoad{}
	if err := bare.Authorize(live.s, live.gm); err != nil {
		t.Errorf("the GM was refused: %v", err)
	}
	if err := bare.Authorize(live.s, live.pc); err == nil {
		t.Error("a player may open a scene")
	} else if e, ok := err.(*Error); !ok || e.Code != CodeForbidden {
		t.Errorf("a player was refused with %v, want forbidden", err)
	}
	if _, err := bare.Apply(live.s, live.gm, live.env); err == nil {
		t.Error("a load with no scene in it replaced the table with nothing")
	}
	if err := bare.Resolve(context.Background(), newLibrary(), live.s); err != nil {
		t.Errorf("resolving a load with no scene in it failed rather than waiting for Apply: %v", err)
	}
}
func TestSceneLoadIsServerSideOnly(t *testing.T) {
	if _, hub := HubCommandPrototypes()["scene.load"]; !hub {
		t.Fatal("scene.load is not in the hub registry")
	}
	_, _, err := DecodeCommand([]byte(`{"type":"scene.load","cid":"1"}`))
	e, ok := err.(*Error)
	if !ok || !strings.Contains(e.Message, "does not know") {
		t.Fatalf("a socket asked for scene.load and got %v; a client that could send a whole State could put anything on the table", err)
	}
}
