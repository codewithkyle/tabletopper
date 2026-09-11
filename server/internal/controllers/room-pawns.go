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
var spawnKinds = map[string]bool{
	pages.RoomSpawnMonsters: true,
	pages.RoomSpawnTokens:   true,
	pages.RoomSpawnNPCs:     true,
}
func (a *App) RoomSpawnFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawn(data))
}
func (a *App) RoomSpawnListFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawnList(data))
}
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
	case len([]rune(name)) > pages.MonsterNameLimit:
		rejectSpawnMonster(w, r, data, "Name must be 128 characters or fewer.")
		return
	}
	hp, ok := spawnMonsterCount(w, r, data, "hp", "Hit points", 1, pages.MonsterHPLimit)
	if !ok {
		return
	}
	ac, ok := spawnMonsterCount(w, r, data, "ac", "Armour class", 0, room.ACLimit)
	if !ok {
		return
	}
	size := pages.NormalizeSize(r.PostFormValue("size"))
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
func rejectSpawnMonster(w http.ResponseWriter, r *http.Request, data pages.RoomSpawnMonsterData, message string) {
	data.Errors = []string{message}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(data.Panel(), data.Errors))
}
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
func (a *App) RoomPawnFragment(w http.ResponseWriter, r *http.Request) {
	a.renderPawnPanel(w, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
}
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
func pawnIDs(w http.ResponseWriter, r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, false
	}
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
func pawnView(pawn *room.Pawn, role room.Role, labels room.PawnLabels, layer string) pages.RoomPawn {
	exact := room.ExactHP(pawn.Kind, labels, role)
	out := pages.RoomPawn{
		ID:     pawn.ID.String(),
		Name:   pawn.Name,
		Image:  pawn.Image,
		Object: pawn.Kind == room.PawnObject,
		HP:     hpText(exact, pawn),
		Layer:  layer,
		Character: pawn.Kind == room.PawnPlayer,
	}
	if pawn.HPBand != nil {
		out.Band = pages.PawnBandText(string(*pawn.HPBand))
	}
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
	if role == room.RoleGM {
		out.Hidden = !pawn.Visible
		if pawn.MonsterID != nil {
			out.MonsterID = pawn.MonsterID.String()
		}
	}
	return out
}
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
func hpText(exact bool, pawn *room.Pawn) string {
	if !exact {
		return ""
	}
	return pages.PawnHPText(pawn.HP, pawn.MaxHP)
}
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
func mayEditPawn(role room.Role, user ulid.ULID, pawn *room.Pawn) bool {
	if pawn == nil {
		return false
	}
	if role == room.RoleGM {
		return true
	}
	return pawn.OwnerID != nil && *pawn.OwnerID == user
}
func hpEntry(r *http.Request, name string) string {
	if entry := strings.TrimSpace(r.FormValue(name + "Entry")); entry != "" {
		return entry
	}
	return r.FormValue(name)
}
func evaluateHP(entry string, current *int, what string) (int, bool, string) {
	value, present, refusal := room.EvaluateHP(entry, current)
	if refusal != "" {
		refusal = what + " " + refusal
	}
	return value, present, refusal
}
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
func (a *App) renderPawnErrors(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, problems []string) {
	renderPanelBlock(w, r, pages.RoomPawnPanel+"-"+pawnID.String(), problems)
}
func (a *App) refusePawnForm(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, action string, err error) {
	var refusal *room.Error
	if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
		a.renderPawnErrors(w, r, pawnID, []string{refusal.Message})
		return
	}
	a.rejectCommand(w, action, err)
}
