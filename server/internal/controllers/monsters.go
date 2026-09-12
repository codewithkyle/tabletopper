package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) MonstersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsters, err := a.monsterList(ctx, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load monsters", "error", err)
		redirectToError(w, r)
		return
	}
	render(w, r, pages.Monsters(pages.MonsterListData{Monsters: monsters}))
}
func (a *App) MonsterListFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.MonsterNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	monsters, err := a.monsterList(ctx, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to search monsters", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterCardsFragment(pages.MonsterListData{Monsters: monsters, Query: term}))
}
func (a *App) monsterList(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MonsterSummary, error) {
	var rows []queries.Monster
	var err error
	if term == "" {
		rows, err = a.Queries.ListMonsters(ctx, ownerID)
	} else {
		rows, err = a.Queries.SearchMonsters(ctx, queries.SearchMonstersParams{
			OwnerID: ownerID,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}
	monsters := make([]pages.MonsterSummary, 0, len(rows))
	for _, row := range rows {
		monsters = append(monsters, monsterSummary(row))
	}
	return monsters, nil
}
func monsterSummary(monster queries.Monster) pages.MonsterSummary {
	image := ""
	if monster.AssetID != nil {
		image = monster.AssetID.String()
	}
	return pages.MonsterSummary{
		ID:       monster.ID.String(),
		Name:     monster.Name,
		Subtitle: monsterSubtitle(monster),
		ImageID:  image,
		CR:       pages.ChallengeRatingLabel(pages.NormalizeChallengeRating(monster.CR)),
		AC:       strconv.FormatUint(uint64(monster.AC), 10),
		HP:       strconv.FormatUint(uint64(monster.HP), 10),
	}
}
func (a *App) MonsterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monster, monsterID, ok := a.loadMonster(w, r)
	if !ok {
		return
	}
	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load monster actions", "error", err)
		redirectToError(w, r)
		return
	}
	render(w, r, pages.EditMonster(monsterToEditPageData(monsterID.String(), monster, actions)))
}
func (a *App) loadMonster(w http.ResponseWriter, r *http.Request) (queries.Monster, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/monsters")
		return queries.Monster{}, ulid.ULID{}, false
	}
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/monsters")
		return queries.Monster{}, ulid.ULID{}, false
	}
	if err != nil {
		slog.Error("Failed to load monster", "error", err)
		redirectToError(w, r)
		return queries.Monster{}, ulid.ULID{}, false
	}
	return monster, monsterID, true
}
func (a *App) MonsterStatBlockFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, err := ulid.Parse(r.URL.Query().Get("monster"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to load monster", "error", err)
		htmx.ServerError(w)
		return
	}
	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load monster actions", "error", err)
		htmx.ServerError(w)
		return
	}
	derived := monsterDerived(monster, actions)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterStatBlockFragment(monsterStatBlock(monster, actions, derived)))
}
func (a *App) MonsterManualFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	params := r.URL.Query()
	if raw := params.Get("monster"); raw != "" {
		monsterID, err := ulid.Parse(raw)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		a.renderManualMonster(w, r, monsterID, sess.UserID)
		return
	}
	term := strings.TrimSpace(params.Get("q"))
	if len([]rune(term)) > pages.MonsterNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	monsters, err := a.monsterList(ctx, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to read the manual for its window", "error", err)
		htmx.ServerError(w)
		return
	}
	data := pages.MonsterListData{Monsters: monsters, Query: term}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if params.Has("q") {
		render(w, r, pages.MonsterManualListFragment(data))
		return
	}
	render(w, r, pages.MonsterManualWindow(data))
}
func (a *App) renderManualMonster(w http.ResponseWriter, r *http.Request, monsterID ulid.ULID, ownerID ulid.ULID) {
	ctx := r.Context()
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to read a monster for the manual window", "error", err)
		htmx.ServerError(w)
		return
	}
	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	})
	if err != nil {
		slog.Error("Failed to read a monster's actions for the manual window", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterManualWindowEntry(monsterStatBlock(monster, actions, monsterDerived(monster, actions))))
}
func monsterToEditPageData(id string, monster queries.Monster, actions []queries.MonsterAction) pages.EditMonsterPageData {
	derived := monsterDerived(monster, actions)
	return pages.EditMonsterPageData{
		MonsterID:                 id,
		Header:                    monsterHeader(monster, derived),
		StatBlock:                 monsterStatBlock(monster, actions, derived),
		Name:                      monster.Name,
		Size:                      pages.NormalizeSize(monster.Size),
		Type:                      pages.NormalizeCreatureType(monster.Type),
		Tags:                      monster.Tags,
		Alignment:                 pages.NormalizeAlignment(monster.Alignment),
		Str:                       strconv.FormatUint(uint64(monster.Str), 10),
		Dex:                       strconv.FormatUint(uint64(monster.Dex), 10),
		Con:                       strconv.FormatUint(uint64(monster.Con), 10),
		Int:                       strconv.FormatUint(uint64(monster.Int), 10),
		Wis:                       strconv.FormatUint(uint64(monster.Wis), 10),
		Cha:                       strconv.FormatUint(uint64(monster.Cha), 10),
		AC:                        strconv.FormatUint(uint64(monster.AC), 10),
		HP:                        strconv.FormatUint(uint64(monster.HP), 10),
		HitDice:                   monster.HitDice,
		Speed:                     monster.Speed,
		InitiativeBonus:           strconv.FormatInt(int64(monster.InitiativeBonus), 10),
		CR:                        pages.NormalizeChallengeRating(monster.CR),
		LegendaryActionUses:       strconv.FormatUint(uint64(monster.LegendaryActionUses), 10),
		LegendaryActionUsesInLair: strconv.FormatUint(uint64(monster.LegendaryActionUsesInLair), 10),
		Vulnerabilities:           monster.Vulnerabilities,
		Resistances:               monster.Resistances,
		Immunities:                monster.Immunities,
		Gear:                      monster.Gear,
		Senses:                    monster.Senses,
		Languages:                 monster.Languages,
		Habitat:                   monster.Habitat,
		Treasure:                  monster.Treasure,
		Description:               monster.Description,
		Derived:                   derived,
		Actions:                   monsterActionRows(actions),
	}
}
func monsterActionRows(actions []queries.MonsterAction) map[string][]pages.MonsterAction {
	rows := map[string][]pages.MonsterAction{}
	for _, action := range actions {
		kind := string(action.Kind)
		rows[kind] = append(rows[kind], monsterActionPageRow(action))
	}
	return rows
}
func (a *App) NewMonsterFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewMonsterFragment())
}
func (a *App) NewMonsterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if problem := parseUploadForm(w, r, imageLimits); problem != nil && problem != errNotMultipart {
		rejectNewMonster(w, r, problem.Message)
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectNewMonster(w, r, "Name is required.")
		return
	case len([]rune(name)) > pages.MonsterNameLimit:
		rejectNewMonster(w, r, "Name must be 128 characters or fewer.")
		return
	}
	picture, filename, ok := a.newMonsterPicture(w, r)
	if !ok {
		return
	}
	id := ulid.Make()
	err := a.Queries.CreateMonsterFromName(ctx, queries.CreateMonsterFromNameParams{
		ID:      id,
		OwnerID: sess.UserID,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to create monster", "error", err)
		htmx.ServerError(w)
		return
	}
	created := name + " has been created."
	if picture != nil {
		if _, err := a.attachMonsterImage(ctx, sess.UserID, id, picture, filename); err != nil {
			slog.Error("Failed to attach the new monster's picture", "error", err, "monsterID", id.String())
			created = name + " has been created, but the picture could not be saved. Add it again from the editor."
		}
	}
	htmx.Toast(w, created)
	htmx.Redirect(w, "/monsters/"+id.String()+"/edit")
}
func (a *App) newMonsterPicture(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	file, filename, problem := openOptionalImageUpload(r, "image", imageLimits)
	if problem != nil {
		rejectNewMonster(w, r, problem.Message)
		return nil, "", false
	}
	if file == nil {
		return nil, "", true
	}
	defer file.Close()
	src, err := decodeUpload(r.Context(), file)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Gave up waiting for a decode slot", "field", "image")
		rejectNewMonster(w, r, "The server is busy. Try again in a moment.")
		return nil, "", false
	}
	if err != nil {
		slog.Warn("Failed to decode a new monster's picture", "error", err)
		rejectNewMonster(w, r, errUnsupportedImage.Message)
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
func rejectNewMonster(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewMonsterPanel, []string{message}))
}
func (a *App) DeleteMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
		return
	}
	monster, err := a.Queries.GetMonsterAsset(ctx, queries.GetMonsterAssetParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to query monster asset", "error", err)
		htmx.ServerError(w)
		return
	}
	if monster.FilePath.Valid {
		if err := a.Storage.Delete(ctx, monster.FilePath.String); err != nil {
			slog.Error("Failed to delete monster image object", "error", err)
			htmx.ServerError(w)
			return
		}
	}
	err = a.tx(ctx, func(q *queries.Queries) error {
		if err := deleteMonsterRows(ctx, q, monsterID, sess.UserID); err != nil {
			return err
		}
		err := q.DeleteMonster(ctx, queries.DeleteMonsterParams{
			ID:      monsterID,
			OwnerID: sess.UserID,
		})
		if err != nil {
			return fmt.Errorf("monster: %w", err)
		}
		if monster.AssetID != nil {
			err := q.DeleteAsset(ctx, queries.DeleteAssetParams{
				ID:      *monster.AssetID,
				OwnerID: sess.UserID,
			})
			if err != nil {
				return fmt.Errorf("image asset: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("Failed to delete monster", "error", err, "monsterID", monsterID.String())
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, monster.Name+" has been deleted.")
}
func deleteMonsterRows(ctx context.Context, q *queries.Queries, monsterID, ownerID ulid.ULID) error {
	if err := q.DeleteMonsterActions(ctx, queries.DeleteMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	}); err != nil {
		return fmt.Errorf("monster actions: %w", err)
	}
	if _, err := q.DeleteMonsterShare(ctx, queries.DeleteMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	}); err != nil {
		return fmt.Errorf("monster share: %w", err)
	}
	return nil
}
