package hub

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

// TURNING A REFERENCE INTO A PAWN. The wire says "this monster", "this token",
// "my character"; a pawn is a name, a picture, a size and a stat line, and
// every one of those is a column. This file is the half of pawn.spawn that has
// a database, and it runs on the caller's goroutine for resolve.go's reason.
//
// FOUR KINDS AND FOUR SOURCES. A monster comes from the GM's manual, a token
// from the GM's library, a character from its owner's sheet, and an object from
// the same library as a token with the picture's own pixel size instead of a
// creature size. What they have in common is the shape of the answer and
// nothing else, which is why this is four functions rather than one with a
// switch inside it.
//
// WHAT IT DOES NOT DECIDE: where the pawn stands, which floor it is on, whether
// players can see it, what its id is, or whether the actor is allowed any of
// this. Those belong to Apply and Authorize, which run after this and have the
// state. This only knows rows.

const (
	// npcHP and npcAC are what a token spawned as a creature arrives with.
	//
	// A TOKEN IS A PICTURE AND HAS NO STAT LINE, so there is nothing to read
	// and something has to be written. One hit point and armour class ten is
	// the least misleading pair available: it is obviously a placeholder rather
	// than a plausible monster, so a GM who meant to fill it in and did not
	// finds out on the first hit rather than after a fight balanced against
	// numbers nobody chose.
	npcHP = 1
	npcAC = 10
)

// resolveSpawn fills in the pawn behind one spawn command.
func (h *Hub) resolveSpawn(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawn) error {
	// RESOLUTION RUNS BEFORE AUTHORIZATION, which is the price of running it
	// off the room's goroutine, and this is where that ordering shows.
	// PawnSpawn.Authorize is the GM and nobody else, so there is nothing here
	// to look up for anybody else: leaving Pawn nil hands the refusal to
	// Authorize, which says the right thing, and a forged spawn off a player's
	// socket costs no query at all.
	//
	// EVERY RESOLVER BELOW THEREFORE ASSUMES THE GM, which is what lets each of
	// them read the acting actor's own library -- the GM is the room's owner,
	// so who.ID IS the manual and the asset list being searched.
	if !who.GM() {
		return nil
	}

	if h.queries == nil {
		return notBuilt("Spawning is not ready", "This server cannot read the library.")
	}

	switch cmd.Kind {
	case room.PawnMonster:
		return h.resolveMonster(ctx, who, cmd)
	case room.PawnNPC:
		return h.resolveToken(ctx, who, cmd)
	case room.PawnObject:
		return h.resolveObject(ctx, roomID, who, cmd)
	case room.PawnPlayer:
		return h.resolveCharacter(ctx, roomID, who, cmd)
	}

	return &room.Error{Code: room.CodeInvalid, Heading: "Bad pawn", Message: "That is not a kind of pawn."}
}

// resolveMonster reads the manual.
//
// THE LIBRARY IS THE ASKER'S OWN, which is resolveMap's rule and holds for the
// same reason: only the GM may put a monster on the table, and the GM is the
// room's owner, so the acting actor's id IS the library being read.
func (h *Hub) resolveMonster(ctx context.Context, who room.Actor, cmd *room.PawnSpawn) error {
	if cmd.MonsterID == nil {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "That spawn named no monster."}
	}

	row, err := h.queries.GetMonsterForRoom(ctx, queries.GetMonsterForRoomParams{
		ID:      *cmd.MonsterID,
		OwnerID: who.ID,
	})
	if err != nil {
		return missing(err, "Monster gone", "That monster is no longer in your manual.")
	}

	// THE SAME NUMBER TWICE, and it is not a mistake. A monster's hit points
	// are its maximum; the pawn is a fresh instance of it, which starts
	// undamaged. The two diverge the moment somebody hits it, and the manual
	// never hears about it -- an instance's damage belongs to the table.
	hp := int(row.HP)
	ac := int(row.AC)

	cmd.Pawn = &room.Pawn{
		Name:      row.Name,
		Image:     imageURL(row.AssetID),
		Size:      creatureSize(row.Size),
		HP:        &hp,
		MaxHP:     &hp,
		AC:        &ac,
		MonsterID: cmd.MonsterID,
	}

	return nil
}

// resolveToken places a token as a creature: an NPC with a picture, a size and
// a placeholder stat line the GM fills in from the pawn's own dialog.
func (h *Hub) resolveToken(ctx context.Context, who room.Actor, cmd *room.PawnSpawn) error {
	asset, err := h.libraryToken(ctx, who, cmd.AssetID)
	if err != nil {
		return err
	}

	name := pawnName(cmd.Name, asset)
	if name == "" {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "That spawn named nothing to place."}
	}

	hp, ac := npcHP, npcAC

	cmd.Pawn = &room.Pawn{
		Name:  name,
		Image: imageURL(assetID(asset)),
		Size:  creatureSize(string(cmd.Size)),
		HP:    &hp,
		MaxHP: &hp,
		AC:    &ac,
	}

	return nil
}

// resolveObject places a token as a prop: a wagon, a boat, a door.
//
// NO STAT LINE AND NO CONDITIONS, which is the object kind's whole shape. Apply
// clears the conditions and the creature size for an object; what is left for
// this to supply is the picture, the name and how big the picture is.
//
// THE SIZE IS THE PICTURE'S AND NOBODY IS ASKED FOR IT. It used to come off the
// wire, from two number fields in the spawn dialog, which asked the GM to
// describe in cells a thing they were looking at -- and got it wrong whenever
// the token was not authored against this table's grid. The assets row already
// records what the picture is, so the answer is read rather than typed, and the
// dialog has two fewer controls. A GM who wants it bigger drags the numbers in
// the pawn's own dialog afterwards, which is where every other thing about a
// pawn is changed.
func (h *Hub) resolveObject(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawn) error {
	if cmd.AssetID == nil {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "An object needs a picture from your library."}
	}

	asset, err := h.libraryToken(ctx, who, cmd.AssetID)
	if err != nil {
		return err
	}

	name := pawnName(cmd.Name, asset)
	if name == "" {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "That spawn named nothing to place."}
	}

	width, height := pictureSize(asset)
	if width == 0 || height == 0 {
		cell := h.oneCell(ctx, roomID)
		width, height = cell, cell
	}

	cmd.Pawn = &room.Pawn{Name: name, Image: imageURL(assetID(asset)), Width: width, Height: height}

	return nil
}

// pictureSize is how much table an object covers: the stored picture's own
// dimensions, read as map pixels. Zero is "this row does not say".
//
// THE COLUMNS ARRIVED WITH THE TILING WORK and library rows written before it
// never got them, so an absent answer is the handful of the developer's own
// rather than a case worth an error message.
func pictureSize(asset *queries.Asset) (int, int) {
	if asset == nil || !asset.Width.Valid || !asset.Height.Valid {
		return 0, 0
	}
	if asset.Width.Int32 < 1 || asset.Height.Int32 < 1 {
		return 0, 0
	}

	return int(asset.Width.Int32), int(asset.Height.Int32)
}

// oneCell is what a picture of unknown size is placed at: one cell of THIS
// table's grid.
//
// IT IS THE SAME FALLBACK THE SPAWN DIALOG DRAWS. The token card carries the
// picture's pixels and leaves the attribute off when the row has none, and the
// client then ghosts one cell -- so the thing under the pointer and the thing
// that lands are the same size, which is the whole reason this reads the grid
// rather than defaulting to 64 on its own.
//
// A ROOM THAT WILL NOT ANSWER FALLS BACK TO THE DEFAULT CELL, which is the only
// number available when there is no room to ask.
func (h *Hub) oneCell(ctx context.Context, roomID ulid.ULID) int {
	view, ok := h.spawn(ctx, roomID)
	if !ok {
		return room.DefaultCellSize
	}

	return max(view.Grid.CellSize, 1)
}

// resolveCharacter places one player's character, which is the GM putting a
// single late arrival on the map rather than the whole party -- the party is
// resolveParty below.
//
// THE STATEMENT IS UNSCOPED AND THE ROOM SUPPLIES THE SCOPE. GetCharacterForRoom
// takes an id and nothing else, because the sheet belongs to a player and the
// person asking for it is the GM. What stands in for an owner check is the seat
// lookup above it: a character id that nobody at this table joined with is not
// found, whoever owns it, so the GM can reach exactly the sheets that walked
// into their room and no others.
func (h *Hub) resolveCharacter(ctx context.Context, roomID ulid.ULID, _ room.Actor, cmd *room.PawnSpawn) error {
	if cmd.CharacterID == nil {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "That spawn named no character."}
	}

	view, ok := h.spawn(ctx, roomID)
	if !ok {
		return errGone
	}

	seat := seatFor(view.Players, *cmd.CharacterID)
	if seat == nil {
		return &room.Error{Code: room.CodeNotFound, Heading: "Character gone", Message: "Nobody at this table joined with that character."}
	}

	row, err := h.queries.GetCharacterForRoom(ctx, *cmd.CharacterID)
	if err != nil {
		return missing(err, "Character gone", "That character no longer exists.")
	}

	cmd.Pawn = characterPawn(row, seat)

	return nil
}

// resolveParty is the Tabletop menu's Spawn pawns: one pawn for everybody who
// is here and has a character, and nobody twice.
//
// THE ROOM DECIDES WHO IS AT THE TABLE, not the request. There is nothing on
// the wire to check, which is why this command has no fields: the player list
// and the pawns already standing on it are both room state, and this reads
// them in one message rather than trusting a browser's idea of either.
//
// THE ROW IS CENTRED ON THE FLOOR AND SPACED A CELL APART, and it is only a
// starting arrangement -- addPawn snaps each one, and the GM drags them where
// the party actually is. A layer with no map centres on the origin, which is
// where the infinite grid's own centre is.
func (h *Hub) resolveParty(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawnCharacters) error {
	// RESOLUTION RUNS BEFORE AUTHORIZATION, the same as resolveSpawn above. A
	// player who forged this would otherwise be told "everybody already has a
	// pawn" -- true, useless, and not the reason they were refused. Leaving
	// Pawns nil hands the refusal to Authorize, which says the right thing.
	if !who.GM() {
		return nil
	}

	if h.queries == nil {
		return notBuilt("Spawning is not ready", "This server cannot read the roster.")
	}

	view, ok := h.spawn(ctx, roomID)
	if !ok {
		return errGone
	}

	seats := make([]room.Player, 0, len(view.Players))
	for _, p := range view.Players {
		if p.Connected && p.CharacterID != nil && !view.Characters[*p.CharacterID] {
			seats = append(seats, p)
		}
	}

	if len(seats) == 0 {
		return &room.Error{
			Code:    room.CodeInvalid,
			Heading: "Nobody to place",
			Message: "Everybody connected with a character already has a pawn on the table.",
		}
	}

	centreX, centreY := 0, 0
	if view.Map != nil {
		centreX, centreY = view.Map.Width/2, view.Map.Height/2
	}

	cell := max(view.Grid.CellSize, 1)
	pawns := make([]room.Pawn, 0, len(seats))

	for i, seat := range seats {
		row, err := h.queries.GetCharacterForRoom(ctx, *seat.CharacterID)
		if err != nil {
			// ONE ABSENT SHEET IS NOT A FAILED BUTTON. Somebody deleted a
			// character while they were sitting at the table; the rest of the
			// party still goes on the map, and the person it happened to is the
			// one who knows why their pawn is missing.
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			return err
		}

		p := *characterPawn(row, &seats[i])
		p.LayerID = view.ActiveLayer
		p.X = centreX + (2*i-(len(seats)-1))*cell/2
		p.Y = centreY

		pawns = append(pawns, p)
	}

	cmd.Pawns = pawns

	return nil
}

// characterPawn is one sheet as a pawn. seat is the player who joined with it,
// which may be absent when a GM places a character belonging to nobody here.
//
// THE PORTRAIT WINS AND THE ACCOUNT PICTURE IS THE FALLBACK. A character with
// a portrait is drawn as that character; one without is drawn as the person
// playing them, which is who everybody at the table is looking for anyway.
//
// AND THE SHARED PLACEHOLDER IS NOT A PICTURE, which is the third step and the
// one that has to be spelled out. An account with no picture of its own carries
// room.DefaultAvatar rather than an empty string, because the player list draws
// an <img> and an <img> needs a URL that resolves. A pawn is not an <img>: the
// canvas draws a disc of the character's initials in the player colour when it
// has nothing, which tells four portrait-less party members apart where four
// copies of the same grey file cannot. So the placeholder is refused here and
// the better placeholder is reached.
//
// EMPTY IS THEREFORE A REAL ANSWER OUT OF THIS FUNCTION, and the client already
// expects it -- an NPC spawned from a name with no token has been arriving that
// way since the spawn dialog existed. See sprites.initials.
func characterPawn(row queries.GetCharacterForRoomRow, seat *room.Player) *room.Pawn {
	hp := int(row.CurrentHP)
	maxHP := int(row.MaxHP)
	ac := int(row.AC)
	owner := row.OwnerID

	image := imageURL(row.AssetID)
	if image == "" && seat != nil && seat.Avatar != room.DefaultAvatar {
		image = seat.Avatar
	}

	id := row.ID

	return &room.Pawn{
		Name:        row.Name,
		Image:       image,
		Size:        creatureSize(row.Size),
		HP:          &hp,
		MaxHP:       &maxHP,
		AC:          &ac,
		OwnerID:     &owner,
		CharacterID: &id,
	}
}

// libraryToken reads a token out of the asker's library. An absent id is not an
// error: an NPC may be a name with no picture at all, which draws as a disc
// with its initials.
func (h *Hub) libraryToken(ctx context.Context, who room.Actor, id *ulid.ULID) (*queries.Asset, error) {
	if id == nil {
		return nil, nil
	}

	asset, err := h.queries.GetLibraryAsset(ctx, queries.GetLibraryAssetParams{
		ID:      *id,
		OwnerID: who.ID,
		Type:    queries.AssetsTypeToken,
	})
	if err != nil {
		return nil, missing(err, "Token gone", "That token is no longer in your library.")
	}

	return &asset, nil
}

// seatFor is the player at this table who joined with a character, or nil.
func seatFor(players []room.Player, character ulid.ULID) *room.Player {
	for i, p := range players {
		if p.CharacterID != nil && *p.CharacterID == character {
			return &players[i]
		}
	}

	return nil
}

// pawnName is the name a spawn dialog typed, falling back to the picture's own.
func pawnName(typed string, asset *queries.Asset) string {
	if name := strings.TrimSpace(typed); name != "" {
		return name
	}
	if asset != nil {
		return strings.TrimSpace(asset.Name)
	}

	return ""
}

func assetID(asset *queries.Asset) *ulid.ULID {
	if asset == nil {
		return nil
	}

	return &asset.ID
}

// imageURL is the route every picture on the table is fetched from, and it is
// deliberately the unscoped one: /assets/images serves a monster's picture to
// any signed-in user for the same reason the tile route serves a map to them.
// What is on the table is shown to the table.
func imageURL(id *ulid.ULID) string {
	if id == nil {
		return ""
	}

	return "/assets/images/" + id.String()
}

// creatureSize normalises a size column into one of the six the protocol knows.
//
// AN UNKNOWN SIZE IS MEDIUM RATHER THAN A REFUSAL. The columns behind this are
// VARCHARs written by an importer and by forms older than this feature, and a
// monster whose size says "Medium " or "" is a row somebody would like to put
// on a table rather than a bug report. Medium is one cell, which is also what
// Size.Footprint answers for anything it does not recognise.
func creatureSize(value string) room.Size {
	size := room.Size(strings.ToLower(strings.TrimSpace(value)))
	if !size.Valid() {
		return room.SizeMedium
	}

	return size
}

// missing turns a no-rows into the protocol's own not-found and leaves every
// other failure as itself, so a database that is down is a 500 and a monster
// somebody deleted is a message.
func missing(err error, heading, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &room.Error{Code: room.CodeNotFound, Heading: heading, Message: message}
	}

	return err
}
