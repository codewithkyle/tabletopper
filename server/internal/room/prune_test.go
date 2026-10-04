package room

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func prunedWorld(t *testing.T) (*world, ulid.ULID, ulid.ULID) {
	t.Helper()
	w := sceneWorld(t)
	token := testID(1300)
	crate := w.spawn(Pawn{Kind: PawnObject, Name: "Crate", Width: 64, Height: 64, X: 10, Y: 10, Visible: true, Image: ImageURL(token)})
	w.spawn(Pawn{
		Kind: PawnMonster, Name: "Goblin", X: 260, Y: 96, Visible: false,
		HP: intp(7), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60)),
	})
	return w, token, crate
}
func countOf(reads []string, kind string) int {
	n := 0
	for _, read := range reads {
		if read == kind {
			n++
		}
	}
	return n
}
func TestPruneDropsWhatTheLibraryNoLongerHolds(t *testing.T) {
	w, _, crate := prunedWorld(t)
	fresh := testID(77)
	lib := newLibrary()
	lib.maps[sceneCellarAsset] = MapRef{AssetID: sceneCellarAsset, Gen: fresh, Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2}
	lib.characters[testCharID] = CharacterInfo{ID: testCharID, OwnerID: testPlayerID, Name: "Ilyana"}
	missing, err := Prune(context.Background(), lib, w.s)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if l := floorCalled(w.s, DefaultLayerName); l.Map != nil {
		t.Error("the ground floor still shows a map that is gone")
	}
	if l := floorCalled(w.s, "Cellar"); l.Map == nil {
		t.Error("the cellar lost a map the library still holds")
	} else if l.Map.Gen != fresh {
		t.Errorf("the cellar is on generation %s, want the library's %s", l.Map.Gen, fresh)
	}
	if len(w.s.Table.Palette) != 0 || len(w.s.Tiles) != 0 {
		t.Errorf("%d pictures and %d tiles survived a library with no terrain", len(w.s.Table.Palette), len(w.s.Tiles))
	}
	for _, p := range w.s.Pawns {
		switch p.Name {
		case "Goblin":
			if p.MonsterID != nil {
				t.Error("a goblin still names a monster the manual no longer holds")
			}
		case "Crate":
			if p.Image != "" {
				t.Errorf("the crate still shows %q", p.Image)
			}
		case "Ari":
			if p.CharacterID == nil || *p.CharacterID != testCharID {
				t.Error("Ari's pawn lost a character that still exists")
			}
		}
	}
	if w.s.Pawn(crate) == nil || !hasPawnNamed(w.s, "Goblin") || !hasPawnNamed(w.s, "Wagon") {
		t.Error("a pawn left the table over a picture or a stat block")
	}
	if seat := w.s.Player(testOtherID); seat.CharacterID != nil || seat.CharacterName != "" {
		t.Errorf("Rin's seat is %+v, want one with no character", seat)
	}
	if seat := w.s.Player(testPlayerID); seat.CharacterID == nil {
		t.Error("Ari's seat lost a character that still exists")
	}
	if w.s.Music.TrackID != nil {
		t.Errorf("the music is %+v after its track was deleted", w.s.Music)
	}
	for _, want := range []string{DefaultLayerName, "Pine forest", "Rolling hills", "Goblin", "Crate", "Rin", "Tavern Brawl"} {
		found := false
		for _, line := range missing {
			if strings.HasPrefix(line, want+": ") {
				found = true
			}
		}
		if !found {
			t.Errorf("nothing in %v says %s went", missing, want)
		}
	}
	for _, line := range missing {
		if strings.HasPrefix(line, "Cellar") || strings.HasPrefix(line, "Ari") || strings.HasPrefix(line, "Wagon") {
			t.Errorf("%q reports something the library still holds", line)
		}
	}
	if got, want := mustJSON(t, w.s), mustJSON(t, w.s.Clone()); got != want {
		t.Error("the pruned state is not normalized")
	}
}
func TestPruneReadsEachThingOnceAndTouchesNothingThatIsThere(t *testing.T) {
	w, token, _ := prunedWorld(t)
	lib := stockedLibrary(testID(50))
	lib.maps[testAssetID] = MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3}
	lib.maps[sceneCellarAsset] = MapRef{AssetID: sceneCellarAsset, Gen: testID(51), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2}
	lib.characters[testCharID] = CharacterInfo{ID: testCharID, OwnerID: testPlayerID, Name: "Ilyana"}
	lib.characters[testOtherChar] = CharacterInfo{ID: testOtherChar, OwnerID: testOtherID, Name: "Brannor"}
	lib.tracks[testTrackID] = TrackInfo{Name: "Tavern Brawl"}
	lib.images[token] = true
	before := mustJSON(t, w.s)
	missing, err := Prune(context.Background(), lib, w.s)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("a full library reports %v", missing)
	}
	if got := mustJSON(t, w.s); got != before {
		t.Errorf("a full library changed the room:\n got %s\nwant %s", got, before)
	}
	if n := countOf(lib.reads, "monster"); n != 1 {
		t.Errorf("the goblin was read %d times for two pawns, want once", n)
	}
	if n := countOf(lib.reads, "character"); n != 2 {
		t.Errorf("the characters were read %d times for two seats and a pawn, want twice", n)
	}
}
func TestPruneKeepsAMapThatIsOnlyNotReady(t *testing.T) {
	w, _, _ := prunedWorld(t)
	lib := stockedLibrary(testID(50))
	lib.maps[sceneCellarAsset] = MapRef{AssetID: sceneCellarAsset, Gen: testID(51), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2}
	held := *floorCalled(w.s, DefaultLayerName).Map
	missing, err := Prune(context.Background(), retiling{lib, testAssetID}, w.s)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	l := floorCalled(w.s, DefaultLayerName)
	if l.Map == nil {
		t.Fatal("a map that is retiling was dropped as if it were deleted")
	}
	if *l.Map != held {
		t.Errorf("the map was rewritten to %+v while retiling, want %+v kept", *l.Map, held)
	}
	for _, line := range missing {
		if strings.HasPrefix(line, DefaultLayerName) {
			t.Errorf("%q reports a map that is merely not ready", line)
		}
	}
}

type retiling struct {
	Library
	asset ulid.ULID
}

func (r retiling) Map(ctx context.Context, asset ulid.ULID) (MapRef, error) {
	if asset == r.asset {
		return MapRef{}, invalid("Map not ready", "That map has not finished tiling yet.")
	}
	return r.Library.Map(ctx, asset)
}
func TestPruneLeavesTheRoomAloneWhenTheLibraryIsBroken(t *testing.T) {
	w, _, _ := prunedWorld(t)
	lib := newLibrary()
	lib.broken = errors.New("the database is down")
	before := mustJSON(t, w.s)
	_, err := Prune(context.Background(), lib, w.s)
	if err == nil {
		t.Fatal("a broken library pruned the room of everything")
	}
	if _, isRoomError := err.(*Error); isRoomError {
		t.Fatalf("a database failure was turned into a refusal: %v", err)
	}
	if got := mustJSON(t, w.s); got != before {
		t.Errorf("a broken library changed the room:\n got %s\nwant %s", got, before)
	}
}
func TestASceneLoadClearsPicturesAndStatBlocksThatAreGone(t *testing.T) {
	w := sceneWorld(t)
	w.spawn(Pawn{Kind: PawnObject, Name: "Crate", Width: 64, Height: 64, X: 10, Y: 10, Visible: true, Image: ImageURL(testID(1300))})
	body, err := w.s.ExportScene()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	scene, err := Unmarshal(body)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	lib := stockedLibrary(testID(77))
	delete(lib.monsters, testID(60))
	cmd := &SceneLoad{Scene: scene}
	if err := cmd.Resolve(context.Background(), lib, liveWorld(t).s); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cmd.Missing) != 2 {
		t.Fatalf("Missing = %v, want the goblin and the crate", cmd.Missing)
	}
	for _, p := range cmd.Scene.Pawns {
		switch p.Name {
		case "Goblin":
			if p.MonsterID != nil {
				t.Error("the goblin still names a monster the manual no longer holds")
			}
		case "Crate":
			if p.Image != "" {
				t.Errorf("the crate still shows %q", p.Image)
			}
		}
	}
	if !hasPawnNamed(cmd.Scene, "Goblin") || !hasPawnNamed(cmd.Scene, "Crate") {
		t.Error("a pawn left the scene over a picture or a stat block")
	}
}
