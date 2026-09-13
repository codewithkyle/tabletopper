package controllers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var (
	testArtID     = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT1")
	testTerrainID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT2")
)

func terrainRows() []queries.Asset {
	return []queries.Asset{
		{ID: testAssetID, Name: "Pine forest", Type: queries.AssetsTypeTerrain},
		{ID: testTerrainID, Name: "Rolling hills", Type: queries.AssetsTypeTerrain},
	}
}

func TestOnlyTheGMFillsTheTilePalette(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.AddToPalette, http.MethodPost, "/rooms/"+testRoomID.String()+"/palette",
		map[string]string{"id": testRoomID.String()},
		url.Values{"asset": {testAssetID.String()}}, memberSession(testRoomID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}
func TestRemovingTerrainNobodyHasIsNotFound(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.RemoveFromPalette, http.MethodDelete,
		"/rooms/"+testRoomID.String()+"/palette/"+testArtID.String(),
		map[string]string{"id": testRoomID.String(), "art": testArtID.String()},
		nil, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}
func TestAnAssetIDThatIsNotOneIsNotAServerError(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.AddToPalette, http.MethodPost, "/rooms/"+testRoomID.String()+"/palette",
		map[string]string{"id": testRoomID.String()},
		url.Values{"asset": {"not-a-ulid"}}, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}
func TestOnlyTheGMSeesTheTilePalette(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.RoomPaletteFragment, http.MethodGet,
		"/fragment/room/palette?room="+testRoomID.String(), nil, nil, memberSession(testRoomID))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}
func TestThePaletteIsAskedForAPartItKnowsOrNoneAtAll(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
	app := tableApp(t, db)
	rec := tableRequest(t, app.RoomPaletteFragment, http.MethodGet,
		"/fragment/room/palette?room="+testRoomID.String()+"&part=everything", nil, nil,
		session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a refused parameter answered with a body: %s", rec.Body.String())
	}
}
func TestTheRingIsThePaletteInTheOrderTheGMBuiltIt(t *testing.T) {
	palette := []room.TileArt{
		{ID: testArtID, AssetID: testAssetID, Name: "Pine forest", Image: "/assets/images/a"},
		{ID: testRoomID, AssetID: testOwnerID, Name: "Rolling hills", Image: "/assets/images/b"},
	}
	ring := tableMenuRing(palette)
	if len(ring) != 2 {
		t.Fatalf("the ring carries %d pictures, want 2", len(ring))
	}
	for i, art := range ring {
		if art.Index != i || art.Count != 2 {
			t.Errorf("picture %d is %d of %d, want %d of 2", i, art.Index, art.Count, i)
		}
	}
	if ring[0].Name != "Pine forest" || ring[1].Image != "/assets/images/b" {
		t.Errorf("the ring is not the palette: %+v", ring)
	}
	if got := tableMenuRing(nil); len(got) != 0 {
		t.Errorf("an empty palette made a ring of %d", len(got))
	}
}
func TestThePaletteWindowMarksWhatIsAlreadyInTheBag(t *testing.T) {
	palette := []room.TileArt{{ID: testArtID, AssetID: testAssetID, Name: "Pine forest"}}
	data := paletteData(testRoomID.String(), "pine", palette, terrainRows())
	if len(data.Entries) != 1 || data.Entries[0].ID != testArtID.String() {
		t.Fatalf("the bag is %+v, want the one entry the room holds", data.Entries)
	}
	if len(data.Shelf) != 2 {
		t.Fatalf("the shelf is %d pictures, want 2", len(data.Shelf))
	}
	if !data.Shelf[0].Held {
		t.Error("the picture already in the bag is not marked")
	}
	if data.Shelf[1].Held {
		t.Error("a picture that is not in the bag is marked as if it were")
	}
	if data.Query != "pine" {
		t.Errorf("the window forgot the term %q", data.Query)
	}
	if !strings.Contains(data.Shelf[1].Image, data.Shelf[1].ID) {
		t.Errorf("a shelf picture does not point at its own asset: %+v", data.Shelf[1])
	}
}
