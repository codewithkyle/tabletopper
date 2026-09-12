package room

import (
	"context"
	"errors"
	"testing"

	"github.com/oklog/ulid/v2"
)

type pictureKey struct {
	id   ulid.ULID
	kind PictureKind
}
type fakeLibrary struct {
	maps       map[ulid.ULID]MapRef
	monsters   map[ulid.ULID]MonsterInfo
	pictures   map[pictureKey]PictureInfo
	characters map[ulid.ULID]CharacterInfo
	broken     error
	reads      []string
}

func (l *fakeLibrary) Map(ctx context.Context, asset ulid.ULID) (MapRef, error) {
	l.reads = append(l.reads, "map")
	if l.broken != nil {
		return MapRef{}, l.broken
	}
	ref, ok := l.maps[asset]
	if !ok {
		return MapRef{}, notFound("Map gone", "That map is no longer in your library.")
	}
	return ref, nil
}
func (l *fakeLibrary) Monster(ctx context.Context, id ulid.ULID) (MonsterInfo, error) {
	l.reads = append(l.reads, "monster")
	if l.broken != nil {
		return MonsterInfo{}, l.broken
	}
	info, ok := l.monsters[id]
	if !ok {
		return MonsterInfo{}, notFound("Monster gone", "That monster is no longer in your manual.")
	}
	return info, nil
}
func (l *fakeLibrary) Picture(ctx context.Context, id ulid.ULID, kind PictureKind) (PictureInfo, error) {
	l.reads = append(l.reads, "picture")
	if l.broken != nil {
		return PictureInfo{}, l.broken
	}
	info, ok := l.pictures[pictureKey{id: id, kind: kind}]
	if !ok {
		return PictureInfo{}, notFound("Picture gone", "That picture is no longer in your library.")
	}
	return info, nil
}
func (l *fakeLibrary) Character(ctx context.Context, id ulid.ULID) (CharacterInfo, error) {
	l.reads = append(l.reads, "character")
	if l.broken != nil {
		return CharacterInfo{}, l.broken
	}
	info, ok := l.characters[id]
	if !ok {
		return CharacterInfo{}, notFound("Character gone", "That character no longer exists.")
	}
	return info, nil
}
func newLibrary() *fakeLibrary {
	return &fakeLibrary{
		maps:       map[ulid.ULID]MapRef{},
		monsters:   map[ulid.ULID]MonsterInfo{},
		pictures:   map[pictureKey]PictureInfo{},
		characters: map[ulid.ULID]CharacterInfo{},
	}
}
func (w *world) resolve(c Resolver, lib Library) {
	w.t.Helper()
	if err := c.Resolve(context.Background(), lib, w.s); err != nil {
		w.t.Fatalf("%T: unexpected refusal: %v", c, err)
	}
}
func (w *world) refuseResolve(c Resolver, lib Library, code string) *Error {
	w.t.Helper()
	err := c.Resolve(context.Background(), lib, w.s)
	if err == nil {
		w.t.Fatalf("%T: expected %s, got no error", c, code)
	}
	e, ok := err.(*Error)
	if !ok {
		w.t.Fatalf("%T: expected a *room.Error, got %T: %v", c, err, err)
	}
	if e.Code != code {
		w.t.Fatalf("%T: expected %s, got %s (%s)", c, code, e.Code, e.Message)
	}
	return e
}

var testMonsterID = testID(1010)

func goblin() MonsterInfo {
	return MonsterInfo{
		Name:  "Goblin",
		Size:  SizeSmall,
		HP:    7,
		AC:    15,
		Image: "/assets/images/" + testAssetID.String(),
	}
}
func TestResolvingAMonsterReadsTheManual(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.monsters[testMonsterID] = goblin()
	cmd := &PawnSpawn{Kind: PawnMonster, Layer: w.layer, MonsterID: idp(testMonsterID)}
	w.resolve(cmd, lib)
	got := cmd.Pawn
	if got.Name != "Goblin" || got.Size != SizeSmall {
		t.Fatalf("the pawn is a %q of size %q", got.Name, got.Size)
	}
	if got.HP == nil || *got.HP != 7 || got.MaxHP == nil || *got.MaxHP != 7 {
		t.Errorf("hit points = %v/%v, want 7/7 -- a fresh instance starts undamaged", got.HP, got.MaxHP)
	}
	if got.AC == nil || *got.AC != 15 {
		t.Errorf("armour class = %v, want 15", got.AC)
	}
	if got.Image != "/assets/images/"+testAssetID.String() {
		t.Errorf("image = %q, want the library's own", got.Image)
	}
	if got.MonsterID == nil || *got.MonsterID != testMonsterID {
		t.Error("the pawn lost the monster it came from, which is what the stat block reads")
	}
}
func TestResolvingAMonsterNobodyOwnsIsNotFound(t *testing.T) {
	w := newWorld(t)
	cmd := &PawnSpawn{Kind: PawnMonster, Layer: w.layer, MonsterID: idp(testMonsterID)}
	w.refuseResolve(cmd, newLibrary(), CodeNotFound)
	if cmd.Pawn != nil {
		t.Error("a refused resolution still built a pawn")
	}
}
func TestResolvingAMonsterWithNoPictureLeavesTheImageEmpty(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	bare := goblin()
	bare.Image = ""
	lib.monsters[testMonsterID] = bare
	cmd := &PawnSpawn{Kind: PawnMonster, Layer: w.layer, MonsterID: idp(testMonsterID)}
	w.resolve(cmd, lib)
	if cmd.Pawn.Image != "" {
		t.Errorf("image = %q, want empty", cmd.Pawn.Image)
	}
}
func TestASpawnThatNamesNothingToReadIsRefusedBeforeTheLibrary(t *testing.T) {
	w := newWorld(t)
	cases := map[string]*PawnSpawn{
		"a monster with no monster":  {Kind: PawnMonster, Layer: w.layer},
		"an object with no picture":  {Kind: PawnObject, Layer: w.layer},
		"a character with nobody":    {Kind: PawnPlayer, Layer: w.layer},
		"a kind nobody has heard of": {Kind: PawnKind("dragonfly"), Layer: w.layer},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			lib := newLibrary()
			w.refuseResolve(cmd, lib, CodeInvalid)
			if len(lib.reads) != 0 {
				t.Errorf("the library was read %v before the command was refused", lib.reads)
			}
			if cmd.Pawn != nil {
				t.Error("a refused resolution still built a pawn")
			}
		})
	}
}
func TestResolvingAnNPCTakesTheStatLineFromTheWire(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureAvatar}] = PictureInfo{
		Name: "Bandit", Image: "/assets/images/" + testAssetID.String(), Width: 256, Height: 256,
	}
	cmd := &PawnSpawn{
		Kind: PawnNPC, Layer: w.layer, AssetID: idp(testAssetID),
		Size: SizeLarge, HP: intp(9), MaxHP: intp(12), AC: intp(13),
	}
	w.resolve(cmd, lib)
	got := cmd.Pawn
	if got.Size != SizeLarge {
		t.Errorf("size = %q, want large", got.Size)
	}
	if got.HP == nil || *got.HP != 9 || got.MaxHP == nil || *got.MaxHP != 12 {
		t.Errorf("hit points = %v/%v, want 9/12", got.HP, got.MaxHP)
	}
	if got.AC == nil || *got.AC != 13 {
		t.Errorf("armour class = %v, want 13", got.AC)
	}
	if got.Name != "Bandit" {
		t.Errorf("name = %q, want the picture's own", got.Name)
	}
}
func TestResolvingAnNPCWithNoStatLineFallsBackToThePlaceholder(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureAvatar}] = PictureInfo{Name: "Innkeeper"}
	cmd := &PawnSpawn{Kind: PawnNPC, Layer: w.layer, AssetID: idp(testAssetID), Size: SizeMedium}
	w.resolve(cmd, lib)
	got := cmd.Pawn
	if got.HP == nil || *got.HP != npcHP || got.MaxHP == nil || *got.MaxHP != npcHP {
		t.Errorf("hit points = %v/%v, want the placeholder", got.HP, got.MaxHP)
	}
	if got.AC == nil || *got.AC != npcAC {
		t.Errorf("armour class = %v, want the placeholder", got.AC)
	}
}
func TestAnNPCTypedNameBeatsThePicturesAndAnUnknownSizeIsMedium(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureAvatar}] = PictureInfo{Name: "Innkeeper"}
	cmd := &PawnSpawn{
		Kind: PawnNPC, Layer: w.layer, AssetID: idp(testAssetID),
		Name: "  Sildar  ", Size: Size("enormous"),
	}
	w.resolve(cmd, lib)
	if cmd.Pawn.Name != "Sildar" {
		t.Errorf("name = %q, want the typed one trimmed", cmd.Pawn.Name)
	}
	if cmd.Pawn.Size != SizeMedium {
		t.Errorf("size = %q, want medium", cmd.Pawn.Size)
	}
}
func TestAnNPCWithNoNameAndNoPictureIsRefused(t *testing.T) {
	w := newWorld(t)
	cmd := &PawnSpawn{Kind: PawnNPC, Layer: w.layer}
	w.refuseResolve(cmd, newLibrary(), CodeInvalid)
}
func TestAnObjectIsTheSizeOfItsPicture(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureToken}] = PictureInfo{
		Name: "Ox-drawn wagon", Image: "/assets/images/" + testAssetID.String(), Width: 512, Height: 171,
	}
	cmd := &PawnSpawn{Kind: PawnObject, Layer: w.layer, AssetID: idp(testAssetID)}
	w.resolve(cmd, lib)
	if cmd.Pawn.Width != 512 || cmd.Pawn.Height != 171 {
		t.Errorf("the wagon is %dx%d, want the picture's 512x171", cmd.Pawn.Width, cmd.Pawn.Height)
	}
	if cmd.Pawn.Name != "Ox-drawn wagon" {
		t.Errorf("name = %q, want the picture's own", cmd.Pawn.Name)
	}
	if cmd.Pawn.HP != nil || cmd.Pawn.AC != nil || cmd.Pawn.Conditions != nil {
		t.Error("an object was given a stat line; it has a size and a picture and nothing else")
	}
}
func TestAnObjectWhosePictureHasNoSizeIsOneCell(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureToken}] = PictureInfo{Name: "Crate"}
	grid := w.s.Table.Grid
	grid.CellSize = 70
	w.apply(&TableSetGrid{Grid: grid}, w.gm)
	cmd := &PawnSpawn{Kind: PawnObject, Layer: w.layer, AssetID: idp(testAssetID)}
	w.resolve(cmd, lib)
	if cmd.Pawn.Width != 70 || cmd.Pawn.Height != 70 {
		t.Errorf("the crate is %dx%d, want one cell of the table it is going on", cmd.Pawn.Width, cmd.Pawn.Height)
	}
}
func TestAnObjectAskingForAnAvatarIsNotFound(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testAssetID, kind: PictureAvatar}] = PictureInfo{Name: "Bandit"}
	cmd := &PawnSpawn{Kind: PawnObject, Layer: w.layer, AssetID: idp(testAssetID)}
	w.refuseResolve(cmd, lib, CodeNotFound)
}
func TestACharacterPawnFallsBackFromPortraitToAccountToNothing(t *testing.T) {
	portrait := "/assets/images/" + testAssetID.String()
	cases := map[string]struct {
		image  string
		avatar string
		want   string
	}{
		"the character's own portrait wins":             {image: portrait, avatar: "https://img.clerk.com/kyle", want: portrait},
		"no portrait falls back to the account picture": {avatar: "https://img.clerk.com/kyle", want: "https://img.clerk.com/kyle"},
		"the shared placeholder is refused":             {avatar: DefaultAvatar, want: ""},
		"a portrait still wins over the placeholder":    {image: portrait, avatar: DefaultAvatar, want: portrait},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.s.Player(testPlayerID).Avatar = tc.avatar
			lib := newLibrary()
			lib.characters[testCharID] = CharacterInfo{
				ID: testCharID, OwnerID: testPlayerID, Name: "Ilyana",
				Size: SizeMedium, HP: 11, MaxHP: 14, AC: 16, Image: tc.image,
			}
			cmd := &PawnSpawn{Kind: PawnPlayer, Layer: w.layer, CharacterID: idp(testCharID)}
			w.resolve(cmd, lib)
			if got := cmd.Pawn.Image; got != tc.want {
				t.Errorf("image = %q, want %q", got, tc.want)
			}
			if cmd.Pawn.OwnerID == nil || *cmd.Pawn.OwnerID != testPlayerID {
				t.Error("the pawn is not owned by the player whose character it is")
			}
			if cmd.Pawn.CharacterID == nil || *cmd.Pawn.CharacterID != testCharID {
				t.Error("the pawn lost the sheet it writes back to")
			}
		})
	}
}
func TestResolvingACharacterNobodyJoinedWithIsNotFound(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	stranger := testID(1011)
	lib.characters[stranger] = CharacterInfo{ID: stranger, Name: "Nobody", Size: SizeMedium}
	cmd := &PawnSpawn{Kind: PawnPlayer, Layer: w.layer, CharacterID: idp(stranger)}
	w.refuseResolve(cmd, lib, CodeNotFound)
	if len(lib.reads) != 0 {
		t.Errorf("the library was read %v for a character nobody is sitting behind", lib.reads)
	}
}
func party() *fakeLibrary {
	lib := newLibrary()
	lib.characters[testCharID] = CharacterInfo{
		ID: testCharID, OwnerID: testPlayerID, Name: "Ilyana", Size: SizeMedium, HP: 11, MaxHP: 14, AC: 16,
	}
	lib.characters[testOtherChar] = CharacterInfo{
		ID: testOtherChar, OwnerID: testOtherID, Name: "Brannor", Size: SizeMedium, HP: 9, MaxHP: 9, AC: 14,
	}
	return lib
}
func TestSpawningThePartyCentresItOnTheMap(t *testing.T) {
	w := newWorld(t)
	w.apply(&TableSetLayerMap{
		Layer:   w.layer,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)
	cmd := &PawnSpawnCharacters{}
	w.resolve(cmd, party())
	if len(cmd.Pawns) != 2 {
		t.Fatalf("the party is %d pawns, want the two seats with a character", len(cmd.Pawns))
	}
	cell := w.s.Table.Grid.CellSize
	for i, p := range cmd.Pawns {
		if p.LayerID != w.layer {
			t.Errorf("%s was placed on another floor", p.Name)
		}
		if p.Y != 2048 {
			t.Errorf("%s stands at y=%d, want the middle of the map", p.Name, p.Y)
		}
		if want := 2048 + (2*i-1)*cell/2; p.X != want {
			t.Errorf("%s stands at x=%d, want %d", p.Name, p.X, want)
		}
	}
}
func TestSpawningThePartyOnABareTableCentresItOnTheOrigin(t *testing.T) {
	w := newWorld(t)
	cmd := &PawnSpawnCharacters{}
	w.resolve(cmd, party())
	for _, p := range cmd.Pawns {
		if p.Y != 0 {
			t.Errorf("%s stands at y=%d, want the origin", p.Name, p.Y)
		}
	}
}
func TestSpawningThePartySkipsACharacterWhoseRowIsGone(t *testing.T) {
	w := newWorld(t)
	lib := party()
	delete(lib.characters, testOtherChar)
	cmd := &PawnSpawnCharacters{}
	w.resolve(cmd, lib)
	if len(cmd.Pawns) != 1 || cmd.Pawns[0].Name != "Ilyana" {
		t.Fatalf("the party is %+v, want the one character that still exists", cmd.Pawns)
	}
}
func TestSpawningThePartyStopsOnALibraryThatIsBroken(t *testing.T) {
	w := newWorld(t)
	lib := party()
	lib.broken = errors.New("the database is on fire")
	cmd := &PawnSpawnCharacters{}
	if err := cmd.Resolve(context.Background(), lib, w.s); err == nil {
		t.Fatal("a broken library was treated as a character nobody has")
	}
	if cmd.Pawns != nil {
		t.Error("a refused resolution still built a party")
	}
}
func TestSpawningThePartyTakesConnectedSeatsWithoutAPawn(t *testing.T) {
	w := newWorld(t)
	w.apply(&PawnSpawn{
		Kind: PawnPlayer, Layer: w.layer, Visible: true, CharacterID: idp(testCharID),
		Pawn: &Pawn{Name: "Ilyana", Size: SizeMedium, CharacterID: idp(testCharID), OwnerID: idp(testPlayerID)},
	}, w.gm)
	cmd := &PawnSpawnCharacters{}
	w.resolve(cmd, party())
	if len(cmd.Pawns) != 1 || cmd.Pawns[0].Name != "Brannor" {
		t.Fatalf("the party is %+v, want the one seat without a pawn", cmd.Pawns)
	}
	w.apply(&PlayerSetConnected{ID: testOtherID, Connected: false}, w.gm)
	away := &PawnSpawnCharacters{}
	w.refuseResolve(away, party(), CodeInvalid)
}
func TestSpawningThePartyWithNobodyToPlaceIsRefusedBeforeTheLibrary(t *testing.T) {
	w := newWorld(t)
	w.apply(&PlayerLeave{ID: testPlayerID}, w.gm)
	w.apply(&PlayerLeave{ID: testOtherID}, w.gm)
	lib := party()
	w.refuseResolve(&PawnSpawnCharacters{}, lib, CodeInvalid)
	if len(lib.reads) != 0 {
		t.Errorf("the library was read %v with nobody to place", lib.reads)
	}
}
func TestResolvingALayersMapTakesWhatTheLibraryGives(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	ref := MapRef{AssetID: testAssetID, Gen: testID(50), Width: 2048, Height: 2048, TileSize: 512, MaxZoom: 2}
	lib.maps[testAssetID] = ref
	cmd := &TableSetLayerMap{Layer: w.layer, AssetID: testAssetID}
	w.resolve(cmd, lib)
	if cmd.Map == nil || *cmd.Map != ref {
		t.Fatalf("the command carries %+v, want the library's own reference", cmd.Map)
	}
}
func TestResolvingAMapThatIsNotInTheLibraryIsNotFound(t *testing.T) {
	w := newWorld(t)
	cmd := &TableSetLayerMap{Layer: w.layer, AssetID: testAssetID}
	w.refuseResolve(cmd, newLibrary(), CodeNotFound)
	if cmd.Map != nil {
		t.Error("a refused resolution still carried a map")
	}
}
func TestEveryResolverIsACommandThatCarriesAResolvedField(t *testing.T) {
	found := 0
	for wire, cmd := range WireCommandPrototypes() {
		if _, ok := cmd.(Resolver); !ok {
			continue
		}
		found++
		if err := (cmd.(Resolver)).Resolve(context.Background(), newLibrary(), NewState(testRoomID, "Room", newEnv())); err == nil {
			t.Errorf("%s resolved against an empty library without refusing", wire)
		}
	}
	if found != 3 {
		t.Fatalf("%d commands resolve, want the map, the spawn and the party", found)
	}
}
