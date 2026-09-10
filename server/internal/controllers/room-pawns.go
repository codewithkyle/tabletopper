package controllers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"log/slog"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// WHAT IS ON THE TABLE, AND THE ROUTES THAT CHANGE IT.
//
// EVERY ONE OF THESE IS HTTP AND NOT A SOCKET COMMAND, which is room-table.go's
// rule for room-table.go's reason: htmx is what this application's controls are
// made of, and a control that posted over the socket would need a second way to
// confirm, a second way to report a refusal, and a second way to draw a form
// with its errors. The socket carries what originates on the CANVAS -- a drag,
// a placement click -- and the DOM carries everything else.
//
// NOTHING HERE AUTHORISES ANYTHING, with one exception. Every command in
// internal/room refuses the wrong actor in its own Authorize, and these
// handlers turn a form into a command and a refusal into the alert modal. The
// exception is the three fragments, which answer a question no command asks --
// "may you LOOK at this" -- and that is where the projection comes in.
//
// THE PROJECTION IS THE ONE THING IN THIS FILE THAT MUST NOT BE GOT WRONG.
// hub.Pawn takes a role and answers with the copy that role may see, or
// nothing. A handler that reached past it would be a door around the whole
// two-audience design: a player guesses a ULID, GETs the panel, and reads a
// hidden monster's hit points that the socket was careful never to send. There
// is deliberately no accessor that hands back the stored pawn.

// spawnKinds is the whole set the spawn dialog accepts, matched before anything
// reaches a statement.
var spawnKinds = map[string]bool{
	pages.RoomSpawnMonsters: true,
	pages.RoomSpawnTokens:   true,
	pages.RoomSpawnNPCs:     true,
}

// RoomSpawnFragment is the Spawn dialog, in the content modal.
func (a *App) RoomSpawnFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawn(data))
}

// RoomSpawnListFragment is the results grid alone, which is what a search
// replaces. It exists for the reason the map picker's list route does: a search
// that swapped the whole dialog would swap the box being typed into.
func (a *App) RoomSpawnListFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawnList(data))
}

// spawnData is the query behind both. A false return has already answered.
func (a *App) spawnData(w http.ResponseWriter, r *http.Request) (pages.RoomSpawnData, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	kind := r.URL.Query().Get("kind")
	if !spawnKinds[kind] {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	data := pages.RoomSpawnData{RoomID: row.ID.String(), Kind: kind, Query: term}

	switch kind {
	case pages.RoomSpawnMonsters:
		monsters, err := a.spawnMonsters(ctx, sess.UserID, term)
		if err != nil {
			slog.Error("Failed to list monsters for the spawn dialog", "error", err)
			htmx.ServerError(w)

			return pages.RoomSpawnData{}, false
		}
		data.Monsters = monsters

	case pages.RoomSpawnNPCs:
		avatars, err := a.spawnAvatars(ctx, sess.UserID, data.RoomID, term)
		if err != nil {
			slog.Error("Failed to list avatars for the spawn dialog", "error", err)
			htmx.ServerError(w)

			return pages.RoomSpawnData{}, false
		}
		data.Avatars = avatars

	default:
		tokens, err := a.spawnTokens(ctx, sess.UserID, term)
		if err != nil {
			slog.Error("Failed to list tokens for the spawn dialog", "error", err)
			htmx.ServerError(w)

			return pages.RoomSpawnData{}, false
		}
		data.Tokens = tokens
	}

	return data, true
}

// spawnMonsters is the manual, whole or searched, as the pick cards read it.
// It goes through monsterSummary so a card in this dialog and a card on the
// manual page are the same five values assembled the same way.
func (a *App) spawnMonsters(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MonsterSummary, error) {
	rows, err := a.monsterRows(ctx, ownerID, term)
	if err != nil {
		return nil, err
	}

	out := make([]pages.MonsterSummary, 0, len(rows))
	for _, m := range rows {
		out = append(out, monsterSummary(m))
	}

	return out, nil
}

func (a *App) monsterRows(ctx context.Context, ownerID ulid.ULID, term string) ([]queries.Monster, error) {
	if term == "" {
		return a.Queries.ListMonsters(ctx, ownerID)
	}

	return a.Queries.SearchMonsters(ctx, queries.SearchMonstersParams{
		OwnerID: ownerID,
		Term:    journalSearchPattern(term),
	})
}

// libraryAssets is one kind of the account's library, whole or searched, which
// is the half the two picture walls below have in common. The type is a
// parameter rather than a second copy of this because it is also the scope: a
// term that matched a map must not put one in a wall of faces.
func (a *App) libraryAssets(ctx context.Context, ownerID ulid.ULID, kind queries.AssetsType, term string) ([]queries.Asset, error) {
	if term == "" {
		return a.Queries.GetLibraryAssets(ctx, queries.GetLibraryAssetsParams{
			OwnerID: ownerID,
			Type:    kind,
		})
	}

	return a.Queries.SearchLibraryAssets(ctx, queries.SearchLibraryAssetsParams{
		OwnerID: ownerID,
		Type:    kind,
		Term:    journalSearchPattern(term),
	})
}

// spawnTokens is the token library, whole or searched.
func (a *App) spawnTokens(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.RoomSpawnToken, error) {
	rows, err := a.libraryAssets(ctx, ownerID, queries.AssetsTypeToken, term)
	if err != nil {
		return nil, err
	}

	out := make([]pages.RoomSpawnToken, 0, len(rows))
	for _, t := range rows {
		out = append(out, pages.RoomSpawnToken{
			ID:     t.ID.String(),
			Name:   t.Name,
			Image:  "/assets/images/" + t.ID.String(),
			Width:  int(t.Width.Int32),
			Height: int(t.Height.Int32),
		})
	}

	return out, nil
}

// spawnAvatars is the face library, whole or searched.
//
// IT IS THE AVATARS AND NOT THE TOKENS, which is the whole of what makes the
// third half a different half. An avatar is a portrait -- a shopkeeper, a
// captain, a cultist -- and a token is a picture of a thing; the manager keeps
// them in two walls for that reason and the dialog follows it. A face carries
// no pixel size because it does not land as a picture: it lands as a creature
// of whatever size the form beside it chose.
func (a *App) spawnAvatars(ctx context.Context, ownerID ulid.ULID, roomID string, term string) ([]pages.RoomSpawnAvatar, error) {
	rows, err := a.libraryAssets(ctx, ownerID, queries.AssetsTypeAvatar, term)
	if err != nil {
		return nil, err
	}

	out := make([]pages.RoomSpawnAvatar, 0, len(rows))
	for _, row := range rows {
		out = append(out, pages.RoomSpawnAvatar{
			RoomID: roomID,
			ID:     row.ID.String(),
			Name:   row.Name,
			Image:  "/assets/images/" + row.ID.String(),
		})
	}

	return out, nil
}

// RoomSpawnNPCFragment is the second step behind one face: the stat line a
// portrait has nowhere to read one from.
//
// IT IS THE WHOLE DIALOG AND NOT THE GRID, which is the kind switch's swap
// rather than the search's. Picking a face changes the heading, the controls
// and the actions, so what comes back is the dialog in its second state --
// see the RoomSpawnNPCData comment for why the visibility switch is on it.
func (a *App) RoomSpawnNPCFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	assetID, err := ulid.Parse(r.URL.Query().Get("asset"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	// THE LIBRARY IS THE ASKER'S OWN AND THE TYPE IS PART OF THE LOOKUP, which
	// is what stops a map's id or a token's being handed to this route and
	// coming back as a face to spawn.
	asset, err := a.Queries.GetLibraryAsset(ctx, queries.GetLibraryAssetParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Type:    queries.AssetsTypeAvatar,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to read an avatar for the spawn dialog", "error", err)
			htmx.ServerError(w)

			return
		}

		w.WriteHeader(http.StatusNotFound)

		return
	}

	data := pages.RoomSpawnNPCData{
		RoomID: row.ID.String(),
		Query:  term,
		Avatar: pages.RoomSpawnAvatar{
			RoomID: row.ID.String(),
			ID:     asset.ID.String(),
			Name:   asset.Name,
			Image:  "/assets/images/" + asset.ID.String(),
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawnNPC(data))
}

// THE DIALOG CAN ADD TO ITSELF, which is what the four routes below are.
//
// NOBODY PREPARES FOR EVERY SESSION. A party talks to a shopkeeper nobody wrote
// down, walks into a room with a cart in it, or picks a fight with something
// the GM invented while they were talking; the alternative to answering that
// from inside the dialog is a second tab, the asset manager, and a table
// waiting. So each wall can add one of its own kind.
//
// WHAT THEY WRITE IS THE ACCOUNT'S AND NOT THE ROOM'S. A token uploaded here is
// on the Tokens page afterwards and a monster written here is in the manual,
// the same as if either had been done a week earlier -- there is no such thing
// as a picture that belongs to one table.
//
// SO WHY IS THE ROOM IN THE PATH? Because the CARD that comes back is this
// dialog's. UploadRoomMap made the same trade a file over: identical work to
// the manager's upload, a different representation afterwards, and the room in
// the URL because the representation names it. An avatar card fetches a form
// whose URL carries the room, and the monster form answers with the whole
// dialog, which is built from it.

// UploadSpawnToken is the Upload token button on the token wall.
func (a *App) UploadSpawnToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.spawnRoom(w, r); !ok {
		return
	}

	card, ok := a.storeLibraryAsset(w, r, tokenKind)
	if !ok {
		return
	}

	htmx.Toast(w, card.Name+" uploaded.")
	render(w, r, pages.RoomSpawnTokenCard(pages.RoomSpawnToken{
		ID:     card.ID,
		Name:   card.Name,
		Image:  "/assets/images/" + card.ID,
		Width:  card.Width,
		Height: card.Height,
	}))
}

// UploadSpawnAvatar is the Upload avatar button on the NPC wall.
func (a *App) UploadSpawnAvatar(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.spawnRoom(w, r)
	if !ok {
		return
	}

	card, ok := a.storeLibraryAsset(w, r, avatarKind)
	if !ok {
		return
	}

	htmx.Toast(w, card.Name+" uploaded.")
	render(w, r, pages.RoomSpawnAvatarCard(pages.RoomSpawnAvatar{
		RoomID: roomID,
		ID:     card.ID,
		Name:   card.Name,
		Image:  "/assets/images/" + card.ID,
	}))
}

// RoomSpawnMonsterFragment is the quick-create form, in the dialog's own slot.
func (a *App) RoomSpawnMonsterFragment(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.spawnRoom(w, r)
	if !ok {
		return
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawnMonster(pages.RoomSpawnMonsterData{RoomID: roomID, Query: term}))
}

// CreateSpawnMonster writes one monster into the manual and answers with the
// monster wall it came from, which now has it on it.
//
// IT ANSWERS WITH THE DIALOG AND NOT WITH ONE CARD, which is the one place
// these four part company. The two uploads leave their wall on screen and
// prepend to it; this form REPLACED the wall, so a card would have nowhere to
// go -- and a GM who has just invented a monster is about to place it, which
// means what they need back is the grid it is now in.
//
// EVERY REFUSAL IS A 422 INTO THE FORM'S OWN ERROR BLOCK, which is the one code
// the dialog carries an hx-status route for. The form is left alone, so what
// was typed is still in it when the message appears above.
func (a *App) CreateSpawnMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	roomID, ok := a.spawnRoom(w, r)
	if !ok {
		return
	}

	data := pages.RoomSpawnMonsterData{RoomID: roomID}

	if problem := parseUploadForm(w, r, imageLimits); problem != nil && problem != errNotMultipart {
		rejectSpawnMonster(w, r, data, problem.Message)

		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectSpawnMonster(w, r, data, "Name is required.")

		return
	// Characters, not bytes: the column is varchar(128) and MySQL counts
	// characters there.
	case len([]rune(name)) > pages.MonsterNameLimit:
		rejectSpawnMonster(w, r, data, "Name must be 128 characters or fewer.")

		return
	}

	hp, ok := spawnMonsterCount(w, r, data, "hp", "Hit points", 1, pages.MonsterHPLimit)
	if !ok {
		return
	}

	// The pawn's limit and not the column's -- see RoomSpawnMonsterData.ACMax.
	ac, ok := spawnMonsterCount(w, r, data, "ac", "Armour class", 0, room.ACLimit)
	if !ok {
		return
	}

	// THE SIZE IS NORMALISED AND NEVER REFUSED, which is NormalizeSize's whole
	// job: the column is free text as far as MySQL is concerned, and a word
	// that is not one of the six is a select somebody edited rather than a
	// message worth writing.
	size := pages.NormalizeSize(r.PostFormValue("size"))

	// THE PICTURE IS DECODED BEFORE THE MONSTER EXISTS, which is
	// newMonsterPicture's rule and holds here for its reason: a file that will
	// not open is the one failure a person can fix, and fixing it means the
	// form is still open with everything else in it, so nothing may have been
	// written by the time they are told.
	picture, filename, ok := a.spawnMonsterPicture(w, r, data)
	if !ok {
		return
	}

	id := ulid.Make()
	err := a.Queries.CreateQuickMonster(ctx, queries.CreateQuickMonsterParams{
		ID:      id,
		OwnerID: sess.UserID,
		Name:    name,
		Size:    size,
		AC:      uint8(ac),
		HP:      uint16(hp),
	})
	if err != nil {
		slog.Error("Failed to create a monster from the spawn dialog", "error", err)
		htmx.ServerError(w)

		return
	}

	// A PICTURE THAT WILL NOT STORE DOES NOT UNDO THE MONSTER. The row is
	// written and the GM is about to place it; what they lose is the face on
	// the card, which the editor can put back, and what they would lose the
	// other way is the monster they just described.
	created := name + " is in your manual."
	if _, err := a.attachMonsterImage(ctx, sess.UserID, id, picture, filename); err != nil {
		slog.Error("Failed to attach a new monster's picture", "error", err, "monsterID", id.String())
		created = name + " is in your manual, but the picture could not be saved. Add it again from the editor."
	}

	monsters, err := a.spawnMonsters(ctx, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to list monsters after a quick create", "error", err)
		htmx.ServerError(w)

		return
	}

	htmx.Toast(w, created)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawn(pages.RoomSpawnData{
		RoomID:   roomID,
		Kind:     pages.RoomSpawnMonsters,
		Monsters: monsters,
	}))
}

// spawnMonsterPicture is the form's file, which is required here and optional
// in the manual's own dialog -- see RoomSpawnMonsterData for why.
func (a *App) spawnMonsterPicture(w http.ResponseWriter, r *http.Request, data pages.RoomSpawnMonsterData) ([]byte, string, bool) {
	file, filename, problem := openOptionalImageUpload(r, "image", imageLimits)
	if problem != nil {
		rejectSpawnMonster(w, r, data, problem.Message)

		return nil, "", false
	}
	if file == nil {
		rejectSpawnMonster(w, r, data, "A picture is required.")

		return nil, "", false
	}
	defer file.Close()

	src, err := decodeUpload(r.Context(), file)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Gave up waiting for a decode slot", "field", "image")
		rejectSpawnMonster(w, r, data, "The server is busy. Try again in a moment.")

		return nil, "", false
	}
	if err != nil {
		slog.Warn("Failed to decode a new monster's picture", "error", err)
		rejectSpawnMonster(w, r, data, errUnsupportedImage.Message)

		return nil, "", false
	}

	encoded, err := images.EncodeWebP(images.Square(src, monsterImageSize))
	if err != nil {
		slog.Error("Failed to encode a new monster's picture as webp", "error", err)
		htmx.ServerError(w)

		return nil, "", false
	}

	return encoded, filename, true
}

// spawnMonsterCount is one of the form's two numbers, bounded by the manual's
// own limit. An empty box is the low end rather than a message: the browser
// refuses it first, and a request that got past that meant the minimum.
func spawnMonsterCount(w http.ResponseWriter, r *http.Request, data pages.RoomSpawnMonsterData, field, caption string, low, high int) (int, bool) {
	raw := strings.TrimSpace(r.PostFormValue(field))
	if raw == "" {
		return low, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < low || value > high {
		rejectSpawnMonster(w, r, data, caption+" must be a whole number between "+strconv.Itoa(low)+" and "+strconv.Itoa(high)+".")

		return 0, false
	}

	return value, true
}

// rejectSpawnMonster answers with the form's error block under a 422.
func rejectSpawnMonster(w http.ResponseWriter, r *http.Request, data pages.RoomSpawnMonsterData, message string) {
	data.Errors = []string{message}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(data.Panel(), data.Errors))
}

// spawnRoom is the check every one of the four starts with: the asker is this
// room's GM, and the room id as the string the cards are built from.
func (a *App) spawnRoom(w http.ResponseWriter, r *http.Request) (string, bool) {
	roomID := r.PathValue("id")
	if roomID == "" {
		roomID = r.URL.Query().Get("room")
	}

	row, _, ok := a.gmTable(r.Context(), r, roomID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return "", false
	}

	return row.ID.String(), true
}

// SpawnParty places a pawn for everybody connected who joined with a character.
// It is the Tabletop menu's Spawn pawns item, and it is the only way a player
// character reaches the table.
//
// THE COMMAND CARRIES NOTHING AND THAT IS THE POINT. Who is at the table and
// which characters are already on it are room state; the hub reads both when it
// resolves this, so there is no list from a browser to be trusted or to have
// gone stale between the page loading and the item being pressed.
//
// A 204 AND NO BODY IS THE WHOLE REPLY. The pawns arrive over the socket as
// pawn.spawned, which is the same way they would arrive for anybody else in the
// room, so there is nothing for this response to swap and nothing for it to
// say. A refusal is the alert modal, out of rejectCommand.
func (a *App) SpawnParty(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnSpawnCharacters{}); err != nil {
		a.rejectCommand(w, "spawn the party", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RoomPawnFragment is the panel that goes in a window, which is the pawn's
// whole surface: what it is, and every control for changing it.
func (a *App) RoomPawnFragment(w http.ResponseWriter, r *http.Request) {
	a.renderPawnPanel(w, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
}

// renderPawnPanel answers with the panel, or with the empty 404 that is the
// only thing a viewer who may not see the pawn is ever told.
//
// NOTHING BUT THE FRAGMENT CALLS IT ANY MORE. The two saves used to answer with
// the panel they had just changed; they answer with its error slot instead,
// because a panel that swapped itself on every autosave would replace whatever
// field the person had moved on to. So the panel is built here, when somebody
// asks for it, and the socket is what tells them to ask again.
func (a *App) renderPawnPanel(w http.ResponseWriter, r *http.Request, roomID, pawnID string) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, pawn, ok := a.livePawn(ctx, r, roomID, pawnID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	view, _ := a.Hub.Table(ctx, row.ID)

	data := pages.RoomPawnData{
		RoomID:  row.ID.String(),
		IsGM:    role == room.RoleGM,
		CanEdit: mayEditPawn(role, sess.UserID, pawn),
		Pawn:    pawnView(pawn, role, tableLabels(view), layerName(view, pawn.LayerID)),
		LayerID: pawn.LayerID.String(),
		Shown:   pawn.Visible,
	}

	// THE FLOOR SELECT IS THE GM'S AND SO IS THE READ BEHIND IT. hub.Table is
	// the room's whole configuration, which a player has no business being
	// handed -- the layer manager is refused to them for the same reason, and
	// PawnSetLayer refuses their move anyway.
	if data.IsGM && view != nil {
		for _, l := range view.Table.Layers {
			data.Layers = append(data.Layers, pages.RoomPawnLayer{
				ID:   l.ID.String(),
				Name: pages.SafeLayerName(l.Name),
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnFragment(data))
}

// RoomConditionRowFragment is one empty condition row, for the form's Add
// button. It is a fragment rather than a clone in JavaScript because the row's
// markup then exists once -- and because server/public/js is not a Tailwind
// source, so a row built there would render with no styling at all.
func (a *App) RoomConditionRowFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	_, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || !mayEditPawn(role, sess.UserID, pawn) || pawn.Kind == room.PawnObject {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnConditionRow(pawn.ID.String(), pages.RoomPawnCondition{
		Color:    string(room.ColorRed),
		Duration: "-1",
		Clear:    "end",
	}))
}

// UpdatePawn is the panel's editor: everything about a pawn except its name,
// which is a dialog, and its hit points, which are the row above it.
//
// IT IS UP TO FOUR COMMANDS AND THEY GO IN ORDER, because the protocol keeps
// them apart on purpose -- conditions are replaced wholesale, visibility is the
// GM's alone and drives the two-audience transitions, and a layer change moves
// a pawn between floors. The plain fields go first: they are the ones that can
// be refused for a value out of range, and a save that had already flipped the
// visibility before failing would leave the GM with half of what they typed.
//
// IT ANSWERS WITH THE FORM'S ERROR SLOT AND NOT WITH THE PANEL, which is the
// shape the character sheet's autosaving panels and the grid form already have.
// The form has no Save button: it posts a few hundred milliseconds after a
// keystroke, which is regularly while somebody is still working in it, and a
// reply that swapped the panel would replace the field they had just tabbed
// into. What brings the window back into step is the socket event, and the
// panel's own trigger declines that only while a typing field has the caret.
func (a *App) UpdatePawn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	pawnID, err := ulid.Parse(r.PathValue("pawn"))
	if err != nil {
		htmx.NotFound(w, "pawn")

		return
	}

	pawn, live := a.Hub.Pawn(ctx, roomID, pawnID, who.Role)
	if !live || !mayEditPawn(who.Role, sess.UserID, pawn) {
		htmx.NotFound(w, "pawn")

		return
	}

	update, problems := pawnUpdateForm(r, pawn)
	if len(problems) > 0 {
		a.renderPawnErrors(w, r, pawnID, problems)

		return
	}

	if err := a.Hub.Dispatch(ctx, roomID, who, update); err != nil {
		a.refusePawnForm(w, r, pawnID, "change a pawn", err)

		return
	}

	if pawn.Kind != room.PawnObject {
		conditions, bad := pawnConditionsForm(r)
		if bad != "" {
			a.renderPawnErrors(w, r, pawnID, []string{bad})

			return
		}

		cmd := &room.PawnSetConditions{ID: pawnID, Conditions: conditions}
		if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
			a.refusePawnForm(w, r, pawnID, "change a pawn's conditions", err)

			return
		}
	}

	// THE TWO GM-ONLY COMMANDS ARE SENT ONLY WHEN THEY CHANGED, and that is not
	// an optimisation. Both emit to players -- a pawn appearing or disappearing
	// from their table -- so sending one that changes nothing is an event
	// everybody reduces to no effect, and for the layer it is a pawn.updated to
	// the GM's every open window as well.
	if who.Role == room.RoleGM {
		if shown := r.FormValue("shown") != ""; shown != pawn.Visible {
			cmd := &room.PawnSetVisible{IDs: []ulid.ULID{pawnID}, Visible: shown}
			if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
				a.refusePawnForm(w, r, pawnID, "hide or reveal a pawn", err)

				return
			}
		}

		if layer, err := ulid.Parse(r.FormValue("layer")); err == nil && layer != pawn.LayerID {
			cmd := &room.PawnSetLayer{IDs: []ulid.ULID{pawnID}, Layer: layer}
			if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
				a.refusePawnForm(w, r, pawnID, "move a pawn between layers", err)

				return
			}
		}
	}

	a.renderPawnErrors(w, r, pawnID, nil)
}

// UpdatePawnHP is the hit-point row: the current total and the maximum, in one
// form because they are one reading -- "4 / 7" is how a table says it.
//
// IT IS SEPARATE FROM THE EDITOR BELOW IT because it is a different gesture at
// a different rate. "The goblin takes 7" is typed every round and takes a sum;
// the fields under it are a size and an armour class somebody sets once. That
// difference is what puts them on different triggers -- change here, a debounced
// keystroke there -- and forms do not nest, so two sibling forms is how a panel
// holds two triggers.
//
// EITHER BOX MAY BE EMPTY AND EMPTY MEANS UNTOUCHED. A pawn can have no hit
// points recorded at all, and a person clearing a box to retype it must not
// have blurred their way into setting the goblin to zero.
//
// IT DOES NOT CLOSE A MODAL. There is no modal open behind this -- it is a
// field in a window -- and sending modal:close would dismiss whatever else the
// GM happened to have open.
func (a *App) UpdatePawnHP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	pawnID, err := ulid.Parse(r.PathValue("pawn"))
	if err != nil {
		htmx.NotFound(w, "pawn")

		return
	}

	pawn, live := a.Hub.Pawn(ctx, roomID, pawnID, who.Role)
	if !live || !mayEditPawn(who.Role, sess.UserID, pawn) {
		htmx.NotFound(w, "pawn")

		return
	}

	hp, hasHP, badHP := evaluateHP(hpEntry(r, "hp"), pawn.HP, "Hit points")
	maxHP, hasMax, badMax := evaluateHP(hpEntry(r, "maxHp"), pawn.MaxHP, "Maximum hit points")

	var problems []string
	for _, bad := range []string{badHP, badMax} {
		if bad != "" {
			problems = append(problems, bad)
		}
	}
	if len(problems) > 0 {
		a.renderPawnErrors(w, r, pawnID, problems)

		return
	}

	// BOTH NUMBERS GO IN ONE COMMAND, which is what makes raising a maximum and
	// healing to it a single entry. PawnUpdate clamps once, after it has applied
	// everything it was given, so a goblin taken from 7/7 to 20/20 is not
	// clipped back to seven on its way through.
	update := &room.PawnUpdate{ID: pawnID}
	if hasHP {
		update.HP = &hp
	}
	if hasMax {
		update.MaxHP = &maxHP
	}

	if hasHP || hasMax {
		if err := a.Hub.Dispatch(ctx, roomID, who, update); err != nil {
			a.refusePawnForm(w, r, pawnID, "change a pawn's hit points", err)

			return
		}
	}

	a.renderPawnErrors(w, r, pawnID, nil)
}

// RenamePawn is the rename dialog's save: one field, one command.
//
// IT DISMISSES THE DIALOG AND ANSWERS NOTHING, which is the content modal's
// contract. The panel behind it is corrected by the socket the same way every
// other open copy of it is -- and it is genuinely behind it, so the refetch is
// not declined for a caret that is in the dialog rather than in the panel.
func (a *App) RenamePawn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	pawnID, err := ulid.Parse(r.PathValue("pawn"))
	if err != nil {
		htmx.NotFound(w, "pawn")

		return
	}

	pawn, live := a.Hub.Pawn(ctx, roomID, pawnID, who.Role)
	if !live || !mayEditPawn(who.Role, sess.UserID, pawn) {
		htmx.NotFound(w, "pawn")

		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		renderPanelBlock(w, r, pages.RoomPawnRenamePanel, []string{"A pawn needs a name."})

		return
	}

	cmd := &room.PawnUpdate{ID: pawnID, Name: &name}
	if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
		var refusal *room.Error
		if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			renderPanelBlock(w, r, pages.RoomPawnRenamePanel, []string{refusal.Message})

			return
		}

		a.rejectCommand(w, "rename a pawn", err)

		return
	}

	htmx.CloseModal(w)
	w.WriteHeader(http.StatusNoContent)
}

// RoomPawnRenameFragment is that dialog, prefilled with the name the pawn has
// now. It is the panel's own permission check again: the projection decides
// whether the asker may see the pawn at all, and mayEditPawn whether they may
// change it.
func (a *App) RoomPawnRenameFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || !mayEditPawn(role, sess.UserID, pawn) {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnRename(pages.RoomPawnRenameData{
		RoomID: row.ID.String(),
		PawnID: pawn.ID.String(),
		Name:   pawn.Name,
	}))
}

// MovePawnsToLayer is the GM sending a selection upstairs. It takes a list
// because the canvas overlay sends one, and the pawn dialog sends a list of one
// rather than there being a second route for it.
func (a *App) MovePawnsToLayer(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	ids, ok := pawnIDs(w, r)
	if !ok {
		return
	}

	layer, err := ulid.Parse(strings.TrimSpace(r.FormValue("layer")))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnSetLayer{IDs: ids, Layer: layer}); err != nil {
		a.rejectCommand(w, "move pawns between layers", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetPawnsShown is the group hide and reveal, from the canvas overlay's toggle.
//
// IT IS THE OVERLAY'S AND NOT THE PANEL'S. A single pawn's visibility is a
// switch inside its own window and rides along with the rest of that form in
// UpdatePawn, which is where every other field of a pawn is saved; this route
// exists for the gesture that has no form -- marquee eight goblins, press once,
// and the ambush is on the table. Both end at the same command with a list.
//
// THE STATE IS ON THE REQUEST RATHER THAN INFERRED. "shown" is present or it is
// not, exactly as the pawn panel's own checkbox posts it, so what the GM saw on
// the button is what they get: a toggle that read the room's own answer here
// would flip twice when two GMs pressed it at once, and land where neither of
// them meant.
//
// A 204 AND NO BODY, because what changes arrives over the socket -- pawn
// updates for the GM, and pawns appearing or disappearing for everybody else.
func (a *App) SetPawnsShown(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	ids, ok := pawnIDs(w, r)
	if !ok {
		return
	}

	cmd := &room.PawnSetVisible{IDs: ids, Visible: r.FormValue("shown") != ""}
	if err := a.Hub.Dispatch(r.Context(), roomID, who, cmd); err != nil {
		a.rejectCommand(w, "hide or reveal pawns", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemovePawns takes pawns off the table, from the dialog with one id or from
// the overlay with a whole selection.
//
// ONE COMMAND WITH EVERY ID AND NOT ONE PER PAWN. PawnRemove drops the
// initiative entries as it goes and emits a single tracker update at the end;
// five commands would have every client re-render the turn order five times to
// reach the same answer.
func (a *App) RemovePawns(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	ids, ok := pawnIDs(w, r)
	if !ok {
		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnRemove{IDs: ids}); err != nil {
		a.rejectCommand(w, "remove pawns", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RoomStatBlockFragment is a monster's page of the manual, opened from a pawn.
//
// IT IS THE GM'S AND NOBODY ELSE'S. The room's monster-health setting exists so
// a table can hide a monster's hit points from its players; a stat block
// carries those, its armour class, its resistances and its legendary actions,
// so serving one to a player would contradict, in a second window, the setting
// the GM chose in the first.
//
// IT RENDERS THE PANEL FRAME AND NOT THE DIALOG ONE. The block goes into a
// window, which is dismissed by the controls on its own title bar, so it ships
// no Close of its own -- see the note on pages.StatBlock.
//
// THE OWNER IS THE ROOM'S AND NOT THE ASKER'S, which is the other half of what
// makes this different from MonsterStatBlockFragment. GetMonster and
// ListMonsterActions are both scoped by owner already, so no new statement is
// needed: what changes is which id goes into the parameter. The pawn is read
// out of the live room first, so the monster reached is one the GM actually put
// on this table rather than any id a request cares to name.
func (a *App) RoomStatBlockFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	row, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || role != room.RoleGM || pawn.MonsterID == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      *pawn.MonsterID,
		OwnerID: row.OwnerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)

		return
	}
	if err != nil {
		slog.Error("Failed to load a pawn's monster", "error", err)
		htmx.ServerError(w)

		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monster.ID,
		OwnerID:   row.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load a pawn's monster actions", "error", err)
		htmx.ServerError(w)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterStatBlockPanel(monsterStatBlock(monster, actions, monsterDerived(monster, actions))))
}

// livePawn is the three questions every fragment in this file asks: is this a
// room this session is in, is the room running, and what does THIS ROLE see of
// that pawn.
//
// THE PAWN COMES BACK PROJECTED OR NOT AT ALL. See hub.Pawn: a player asking
// about a hidden pawn, or one on another floor, gets nothing -- which the
// caller answers with an empty 404, the same answer a pawn id that never
// existed gets. The two are indistinguishable on purpose.
func (a *App) livePawn(ctx context.Context, r *http.Request, roomID string, pawnID string) (queries.GetRoomRow, room.Role, *room.Pawn, bool) {
	sess := session.FromContext(ctx)

	row, role, err := a.roomMember(ctx, sess, roomID)
	if err != nil || a.Hub == nil {
		return queries.GetRoomRow{}, "", nil, false
	}

	id, err := ulid.Parse(pawnID)
	if err != nil {
		return queries.GetRoomRow{}, "", nil, false
	}

	pawn, ok := a.Hub.Pawn(ctx, row.ID, id, role)
	if !ok {
		return queries.GetRoomRow{}, "", nil, false
	}

	return row, role, pawn, true
}

// pawnActor is the mutation half of the same check: who is asking, about which
// room. It writes the whole response on failure.
func (a *App) pawnActor(w http.ResponseWriter, r *http.Request) (room.Actor, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if a.Hub == nil {
		htmx.NotFound(w, "room")

		return room.Actor{}, ulid.ULID{}, false
	}

	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")

		return room.Actor{}, ulid.ULID{}, false
	}

	return room.Actor{ID: sess.UserID, Role: role}, row.ID, true
}

// pawnIDs reads the repeated ids field the two list routes take, bounded by the
// protocol's own selection limit. Anything else is an empty 404: the only thing
// that produces one is a request this server did not write.
//
// ParseForm IS WHAT MAKES ONE READ SERVE BOTH VERBS, and the reason is a
// property of each side. htmx puts hx-vals in the QUERY STRING for GET and
// DELETE and in the BODY for everything else -- `/GET|DELETE/.test(method)` in
// its own source -- so the removal arrives one way and the layer move the
// other. net/http's ParseForm merges the query into r.Form for every method and
// the body only for POST, PUT and PATCH, which is exactly the union of the two.
// Reading r.PostForm here instead would work for the layer move and find
// nothing at all for the removal.
func pawnIDs(w http.ResponseWriter, r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)

		return nil, false
	}

	// COMMAS AS WELL AS REPEATS, and the commas are what the canvas overlay
	// sends. htmx's hx-vals SETS each key rather than appending it, so an array
	// arrives as one value with the elements joined -- there is no way to make
	// it emit a repeated field. A ULID has no comma in it, so splitting is
	// exact, and the pawn dialog's single id goes through the same path
	// unchanged.
	var raw []string
	for _, value := range r.Form["ids"] {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				raw = append(raw, part)
			}
		}
	}

	if len(raw) == 0 || len(raw) > room.SelectionMax {
		w.WriteHeader(http.StatusNotFound)

		return nil, false
	}

	ids := make([]ulid.ULID, 0, len(raw))
	for _, value := range raw {
		id, err := ulid.Parse(value)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)

			return nil, false
		}
		ids = append(ids, id)
	}

	return ids, true
}

// pawnView turns the projected pawn into strings, which is the last place
// anything is decided about what a viewer is told.
//
// EVERY WITHHELD VALUE IS ALREADY nil BY THE TIME IT GETS HERE, so the empty
// strings below are a consequence of the projection rather than a second copy
// of it. A player looking at a monster in a band room arrives with HP nil,
// MaxHP nil and HPBand set, and there is nothing in this function that could
// put a number back.
// pawnView is the panel's copy of a pawn. It is built from the PROJECTED pawn,
// so everything on it has already been through projectPawn -- and since that
// stopped withholding hit points, this is where the room's label setting is
// obeyed for the details window. See ExactHP in internal/room/state.go: the
// numbers are in the response either way, and this decides whether they are in
// the markup.
func pawnView(pawn *room.Pawn, role room.Role, labels room.PawnLabels, layer string) pages.RoomPawn {
	exact := room.ExactHP(pawn.Kind, labels, role)

	out := pages.RoomPawn{
		ID:     pawn.ID.String(),
		Name:   pawn.Name,
		Image:  pawn.Image,
		Object: pawn.Kind == room.PawnObject,
		HP:     hpText(exact, pawn),
		Layer:  layer,

		// The kind, narrowed to the one question the panel asks of it: is this
		// somebody's character. It is not the kind itself, because a template
		// holding a protocol value would be a second place the enum lives.
		Character: pawn.Kind == room.PawnPlayer,
	}

	if pawn.HPBand != nil {
		out.Band = pages.PawnBandText(string(*pawn.HPBand))
	}
	// AND THE BOXES GO WITH THE READING. They are the same two numbers in an
	// editable shape, so a viewer who is not shown the line is not shown the
	// fields either -- and anybody who may edit a pawn is shown them by
	// definition, because a GM reads everything and a player's own character is
	// never banded.
	if exact && pawn.HP != nil {
		out.HPValue = strconv.Itoa(*pawn.HP)
	}
	if exact && pawn.MaxHP != nil {
		out.MaxHP = strconv.Itoa(*pawn.MaxHP)
	}
	if pawn.AC != nil {
		out.AC = strconv.Itoa(*pawn.AC)
	}

	if out.Object {
		out.Pixels = pages.PawnPixelsText(pawn.Width, pawn.Height, pawn.Rotation)
		out.Width = strconv.Itoa(pawn.Width)
		out.Height = strconv.Itoa(pawn.Height)
		out.Rotation = strconv.Itoa(pawn.Rotation)
	} else {
		out.Size = pages.PawnSizeText(string(pawn.Size))
		out.SizeValue = string(pawn.Size)
	}

	out.Conditions = pawnConditions(pawn.Conditions)

	// THE TWO GM-ONLY FIELDS, and they are set here rather than in the markup
	// because a template that decided them would be a second place the rule
	// lives. A player never receives a hidden pawn at all, so the first is
	// always false on their copy anyway; the second is what draws the stat
	// block button, which is the GM's alone.
	if role == room.RoleGM {
		out.Hidden = !pawn.Visible
		if pawn.MonsterID != nil {
			out.MonsterID = pawn.MonsterID.String()
		}
	}

	return out
}

// layerName is the floor a pawn stands on, which is worth showing because a
// pawn's window outlives the GM's view of its floor. An unavailable table is an
// empty string rather than a guess -- the panel simply omits the line.
// pawnConditions is one pawn's conditions as chips and rows. It is shared with
// the initiative strip, which prints the same chips on the acting line: two
// copies of this mapping would be two places for a duration to be formatted
// differently in.
func pawnConditions(conditions []room.Condition) []pages.RoomPawnCondition {
	out := make([]pages.RoomPawnCondition, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, pages.RoomPawnCondition{
			ID:           c.ID.String(),
			Name:         c.Name,
			Color:        string(c.Color),
			Duration:     pages.PawnDurationValue(c.Duration),
			DurationText: pages.PawnDurationText(c.Duration),
			Clear:        string(c.Clear),
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// hpText is the printed line, and the whole of what the setting changes here:
// the numbers when the viewer is shown them, and nothing when they are not --
// in which case Band carries the word instead, or is empty in a room that
// labels nothing.
func hpText(exact bool, pawn *room.Pawn) string {
	if !exact {
		return ""
	}

	return pages.PawnHPText(pawn.HP, pawn.MaxHP)
}

// tableLabels is the room's setting, or the default when the table could not be
// read. The fallback is the SAFE one rather than the permissive one: a panel
// built without knowing the room shows a player the word.
func tableLabels(view *hub.TableView) room.PawnLabels {
	if view == nil {
		return room.LabelsDefault
	}

	return view.Table.PawnLabels
}

func layerName(view *hub.TableView, id ulid.ULID) string {
	if view == nil {
		return ""
	}

	for _, l := range view.Table.Layers {
		if l.ID == id {
			return pages.SafeLayerName(l.Name)
		}
	}

	return ""
}

// mayEditPawn is the courtesy that decides which controls are drawn, and it is
// the same rule PawnUpdate.Authorize applies again on every post: the GM, or
// the player the pawn belongs to.
func mayEditPawn(role room.Role, user ulid.ULID, pawn *room.Pawn) bool {
	if pawn == nil {
		return false
	}
	if role == room.RoleGM {
		return true
	}

	return pawn.OwnerID != nil && *pawn.OwnerID == user
}

// hpEntry is what one hit-point box asked for: the change it carried when it
// carried one, and the number in it otherwise.
//
// THE CHANGE TRAVELS BESIDE THE NUMBER, in a hidden twin the panel renders and
// js/room/hp.ts fills. The box shows the resolved number so the person sees it
// the moment they leave the field, but that number was counted from the one
// last RENDERED -- and the panel declines refetches while a box has the caret,
// so the box can be reading 16 while this room holds 10 because a player just
// edited their own sheet. Applying "-4" to the room's 10 is the right answer;
// applying the box's 12 would have been a lost update on the one field that is
// typed every round. A request without the twin -- the script did not run, or
// the entry was a number -- reads the box, as it always did.
func hpEntry(r *http.Request, name string) string {
	if entry := strings.TrimSpace(r.FormValue(name + "Entry")); entry != "" {
		return entry
	}

	return r.FormValue(name)
}

// evaluateHP is the arithmetic a hit-point box takes, and it is the core's:
// see room.EvaluateHP. The wrapper puts the box's caption on the refusal.
func evaluateHP(entry string, current *int, what string) (int, bool, string) {
	value, present, refusal := room.EvaluateHP(entry, current)
	if refusal != "" {
		refusal = what + " " + refusal
	}

	return value, present, refusal
}

// pawnUpdateForm turns the editor into the plain-field command. Every field is
// a pointer, so a value the form did not carry is left alone rather than reset
// -- which is what lets the object form omit a creature size and the creature
// form omit a width and a height.
//
// THE NAME AND THE HIT POINTS ARE NOT IN IT, and both are absences worth
// stating. The name is the rename dialog's, and a form that read a missing
// field as an empty one would refuse every autosave with "a pawn needs a name".
// The two hit-point boxes are their own form on their own route, because they
// take arithmetic and fire on a different event.
func pawnUpdateForm(r *http.Request, pawn *room.Pawn) (*room.PawnUpdate, []string) {
	var problems []string

	cmd := &room.PawnUpdate{ID: pawn.ID}

	if value, ok, bad := optionalNumber(r.FormValue("ac"), "Armour class"); bad != "" {
		problems = append(problems, bad)
	} else if ok {
		cmd.AC = &value
	}

	if pawn.Kind == room.PawnObject {
		width, wBad := requiredNumber(r.FormValue("width"), "Width")
		height, hBad := requiredNumber(r.FormValue("height"), "Height")
		if wBad != "" {
			problems = append(problems, wBad)
		}
		if hBad != "" {
			problems = append(problems, hBad)
		}
		if wBad == "" && hBad == "" {
			cmd.Width, cmd.Height = &width, &height
		}

		// THE ANGLE IS FOLDED RATHER THAN REFUSED, so a GM who types 400 gets a
		// wagon at 40 degrees rather than a form back with a complaint about a
		// number that means exactly what they wanted. The input's own min and
		// max keep an ordinary entry inside one turn; this is what happens when
		// somebody goes round the input.
		if rotation, bad := requiredNumber(r.FormValue("rotation"), "Angle"); bad != "" {
			problems = append(problems, bad)
		} else {
			cmd.Rotation = &rotation
		}

		return cmd, problems
	}

	size := room.Size(strings.TrimSpace(r.FormValue("size")))
	if !size.Valid() {
		problems = append(problems, "Pick one of the six creature sizes.")
	} else {
		cmd.Size = &size
	}

	return cmd, problems
}

// pawnConditionsForm reads the repeater's parallel fields. Every row emits all
// four, so the four slices line up by index; a row whose name was left blank is
// somebody who added one and changed their mind, and is dropped rather than
// refused.
func pawnConditionsForm(r *http.Request) ([]room.Condition, string) {
	ids := r.Form["conditionId"]
	names := r.Form["conditionName"]
	colors := r.Form["conditionColor"]
	durations := r.Form["conditionDuration"]
	clears := r.Form["conditionClear"]

	if len(names) != len(colors) || len(names) != len(durations) || len(names) != len(clears) || len(names) != len(ids) {
		return nil, "That form could not be read. Close it and try again."
	}

	out := make([]room.Condition, 0, len(names))
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		duration, err := strconv.Atoi(strings.TrimSpace(durations[i]))
		if err != nil {
			return nil, "A condition's turns must be a number, or -1 until it is removed."
		}

		condition := room.Condition{
			Name:     name,
			Color:    room.ConditionColor(colors[i]),
			Duration: duration,
			Clear:    room.ClearTrigger(clears[i]),
		}

		// AN EMPTY ID IS A CHIP SOMEBODY HAS JUST INVENTED and the server mints
		// one. A row that already had an id keeps it, so a duration ticking
		// down does not look like a different condition every round.
		if raw := strings.TrimSpace(ids[i]); raw != "" {
			id, err := ulid.Parse(raw)
			if err != nil {
				return nil, "That form could not be read. Close it and try again."
			}
			condition.ID = id
		}

		out = append(out, condition)
	}

	return out, ""
}

// optionalNumber is a field that may be left empty, which is how a pawn with no
// armour class stays that way.
func optionalNumber(entry string, what string) (int, bool, string) {
	text := strings.TrimSpace(entry)
	if text == "" {
		return 0, false, ""
	}

	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, what + " must be a number."
	}

	return value, true, ""
}

func requiredNumber(entry string, what string) (int, string) {
	value, err := strconv.Atoi(strings.TrimSpace(entry))
	if err != nil {
		return 0, what + " must be a number."
	}

	return value, ""
}

// renderPawnErrors puts a refusal above the fields that caused it rather than
// in the alert modal.
//
// ONE SLOT NOW SERVES BOTH FORMS, because both are in the same panel: the quick
// hit-point control and the Save beneath it write into the same block, and a
// second slot would be a message that appeared somewhere the eye was not.
//
// THE 422 IS DELIBERATE AND SO IS THE hx-status:422 BESIDE IT. The page's
// noSwap config swallows every 4xx, which is right for a mutation whose answer
// is a dialog; a form with fields needs its errors on screen, so both forms
// carry an override naming that slot. It is the shape the character panels and
// the grid form already have.
func (a *App) renderPawnErrors(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, problems []string) {
	renderPanelBlock(w, r, pages.RoomPawnPanel+"-"+pawnID.String(), problems)
}

// refusePawnForm turns a refusal from the protocol into a form error where the
// protocol is complaining about a value, and into the alert modal where it is
// complaining about anything else -- a pawn that is gone, an actor who may not.
func (a *App) refusePawnForm(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, action string, err error) {
	var refusal *room.Error
	if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
		a.renderPawnErrors(w, r, pawnID, []string{refusal.Message})

		return
	}

	a.rejectCommand(w, action, err)
}
