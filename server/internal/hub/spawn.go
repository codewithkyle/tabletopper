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






















const (
	
	
	
	
	
	
	
	
	
	
	
	
	
	npcHP = 1
	npcAC = 10
)


func (h *Hub) resolveSpawn(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawn) error {
	
	
	
	
	
	
	
	
	
	
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
		return h.resolveNPC(ctx, who, cmd)
	case room.PawnObject:
		return h.resolveObject(ctx, roomID, who, cmd)
	case room.PawnPlayer:
		return h.resolveCharacter(ctx, roomID, who, cmd)
	}

	return &room.Error{Code: room.CodeInvalid, Heading: "Bad pawn", Message: "That is not a kind of pawn."}
}






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














func (h *Hub) resolveNPC(ctx context.Context, who room.Actor, cmd *room.PawnSpawn) error {
	asset, err := h.libraryPicture(ctx, who, cmd.AssetID, queries.AssetsTypeAvatar)
	if err != nil {
		return err
	}

	name := pawnName(cmd.Name, asset)
	if name == "" {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "That spawn named nothing to place."}
	}

	hp, maxHP, ac := npcHP, npcHP, npcAC
	if cmd.MaxHP != nil {
		maxHP = *cmd.MaxHP
		hp = maxHP
	}
	if cmd.HP != nil {
		hp = *cmd.HP
	}
	if cmd.AC != nil {
		ac = *cmd.AC
	}

	cmd.Pawn = &room.Pawn{
		Name:  name,
		Image: imageURL(assetID(asset)),
		Size:  creatureSize(string(cmd.Size)),
		HP:    &hp,
		MaxHP: &maxHP,
		AC:    &ac,
	}

	return nil
}















func (h *Hub) resolveObject(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawn) error {
	if cmd.AssetID == nil {
		return &room.Error{Code: room.CodeInvalid, Heading: "Nothing to place", Message: "An object needs a picture from your library."}
	}

	asset, err := h.libraryPicture(ctx, who, cmd.AssetID, queries.AssetsTypeToken)
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







func pictureSize(asset *queries.Asset) (int, int) {
	if asset == nil || !asset.Width.Valid || !asset.Height.Valid {
		return 0, 0
	}
	if asset.Width.Int32 < 1 || asset.Height.Int32 < 1 {
		return 0, 0
	}

	return int(asset.Width.Int32), int(asset.Height.Int32)
}












func (h *Hub) oneCell(ctx context.Context, roomID ulid.ULID) int {
	view, ok := h.spawn(ctx, roomID)
	if !ok {
		return room.DefaultCellSize
	}

	return max(view.Grid.CellSize, 1)
}











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













func (h *Hub) resolveParty(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd *room.PawnSpawnCharacters) error {
	
	
	
	
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








func (h *Hub) libraryPicture(ctx context.Context, who room.Actor, id *ulid.ULID, kind queries.AssetsType) (*queries.Asset, error) {
	if id == nil {
		return nil, nil
	}

	asset, err := h.queries.GetLibraryAsset(ctx, queries.GetLibraryAssetParams{
		ID:      *id,
		OwnerID: who.ID,
		Type:    kind,
	})
	if err != nil {
		return nil, missing(err, "Picture gone", "That picture is no longer in your library.")
	}

	return &asset, nil
}


func seatFor(players []room.Player, character ulid.ULID) *room.Player {
	for i, p := range players {
		if p.CharacterID != nil && *p.CharacterID == character {
			return &players[i]
		}
	}

	return nil
}


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





func imageURL(id *ulid.ULID) string {
	if id == nil {
		return ""
	}

	return "/assets/images/" + id.String()
}








func creatureSize(value string) room.Size {
	size := room.Size(strings.ToLower(strings.TrimSpace(value)))
	if !size.Valid() {
		return room.SizeMedium
	}

	return size
}




func missing(err error, heading, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &room.Error{Code: room.CodeNotFound, Heading: heading, Message: message}
	}

	return err
}
