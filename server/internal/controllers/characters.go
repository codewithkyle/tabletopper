package controllers
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
func (a *App) DeleteCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}
	character, err := a.Queries.GetCharacterAsset(ctx, queries.GetCharacterAssetParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "character")
		return
	}
	if err != nil {
		slog.Error("Failed to query character asset", "error", err)
		htmx.ServerError(w)
		return
	}
	imageKeys, err := a.Queries.ListCharacterJournalImages(ctx, queries.ListCharacterJournalImagesParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to list character journal images", "error", err)
		htmx.ServerError(w)
		return
	}
	if err := a.Storage.DeleteMany(ctx, imageKeys); err != nil {
		slog.Error("Failed to delete journal image objects", "error", err, "count", len(imageKeys))
		htmx.ServerError(w)
		return
	}
	if character.FilePath.Valid {
		if err := a.Storage.Delete(ctx, character.FilePath.String); err != nil {
			slog.Error("Failed to delete avatar object", "error", err)
			htmx.ServerError(w)
			return
		}
	}
	err = a.tx(ctx, func(q *queries.Queries) error {
		if err := deleteCharacterRows(ctx, q, characterID, sess.UserID); err != nil {
			return err
		}
		err := q.DeleteCharacter(ctx, queries.DeleteCharacterParams{
			ID:      characterID,
			OwnerID: sess.UserID,
		})
		if err != nil {
			return fmt.Errorf("character: %w", err)
		}
		if character.AssetID != nil {
			err := q.DeleteAsset(ctx, queries.DeleteAssetParams{
				ID:      *character.AssetID,
				OwnerID: sess.UserID,
			})
			if err != nil {
				return fmt.Errorf("portrait asset: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("Failed to delete character", "error", err, "characterID", characterID.String())
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, character.Name+" has been deleted.")
}
func deleteCharacterRows(ctx context.Context, q *queries.Queries, characterID, ownerID ulid.ULID) error {
	if err := q.DeleteCharacterJournalImages(ctx, queries.DeleteCharacterJournalImagesParams{
		OwnerID:     ownerID,
		CharacterID: characterID,
	}); err != nil {
		return fmt.Errorf("journal images: %w", err)
	}
	if err := q.DeleteCharacterJournals(ctx, queries.DeleteCharacterJournalsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("journals: %w", err)
	}
	if err := q.DeleteCharacterInventory(ctx, queries.DeleteCharacterInventoryParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	if err := q.DeleteCharacterAttacks(ctx, queries.DeleteCharacterAttacksParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("attacks: %w", err)
	}
	if err := q.DeleteCharacterSpells(ctx, queries.DeleteCharacterSpellsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("spells: %w", err)
	}
	if err := q.DeleteCharacterSpellSlots(ctx, queries.DeleteCharacterSpellSlotsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("spell slots: %w", err)
	}
	if err := q.DeleteSharesForCharacter(ctx, queries.DeleteSharesForCharacterParams{
		CharacterID: &characterID,
		OwnerID:     ownerID,
	}); err != nil {
		return fmt.Errorf("shares: %w", err)
	}
	return nil
}
func (a *App) CharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	character, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}
	equipped, err := a.Queries.ListEquippedInventory(ctx, queries.ListEquippedInventoryParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load equipped inventory", "error", err)
		redirectToError(w, r)
		return
	}
	attacks, err := a.Queries.ListCharacterAttacks(ctx, queries.ListCharacterAttacksParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load attacks", "error", err)
		redirectToError(w, r)
		return
	}
	prepared, err := a.Queries.ListPreparedSpells(ctx, queries.ListPreparedSpellsParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load prepared spells", "error", err)
		redirectToError(w, r)
		return
	}
	levels, ok := a.loadSpellLevels(w, r, characterID, sess.UserID)
	if !ok {
		return
	}
	data := characterToEditPageData(characterID.String(), character)
	data.Attacks = attackPageRows(attacks)
	data.Equipped = inventoryPageItems(equipped)
	data.Prepared = preparedSpellGroups(prepared)
	data.SpellSlots = levels
	render(w, r, pages.EditCharacter(data))
}
func (a *App) loadCharacter(w http.ResponseWriter, r *http.Request) (queries.Character, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	id := r.PathValue("id")
	if id == "" {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}
	uid, err := ulid.Parse(id)
	if err != nil {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}
	character, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      uid,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return queries.Character{}, ulid.ULID{}, false
	}
	if err != nil {
		slog.Error("Failed to load character", "error", err)
		redirectToError(w, r)
		return queries.Character{}, ulid.ULID{}, false
	}
	return character, uid, true
}
func (a *App) CharactersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	results, err := a.Queries.GetCharacters(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to load characters", "error", err)
		redirectToError(w, r)
		return
	}
	render(w, r, pages.Characters(results))
}
func (a *App) FeatureRowFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.FeatureRowFragment())
}
const characterNameLimit = 128
func (a *App) NewCharacterFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewCharacterFragment())
}
func (a *App) NewCharacterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if err := r.ParseForm(); err != nil {
		rejectNewCharacter(w, r, "The submitted form data could not be read.")
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectNewCharacter(w, r, "Name is required.")
		return
	case len([]rune(name)) > characterNameLimit:
		rejectNewCharacter(w, r, "Name must be 128 characters or fewer.")
		return
	}
	id := ulid.Make()
	err := a.Queries.CreateCharacterFromName(ctx, queries.CreateCharacterFromNameParams{
		ID:      id,
		OwnerID: sess.UserID,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to create character", "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, name+" has been created.")
	htmx.Redirect(w, "/characters/"+id.String()+"/edit")
}
func rejectNewCharacter(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewCharacterPanel, []string{message}))
}
func characterToEditPageData(id string, character queries.Character) pages.EditCharacterPageData {
	derived := characterDerived(character)
	return pages.EditCharacterPageData{
		CharacterID:         id,
		Header:              characterHeaderFrom(character, derived),
		Name:                character.Name,
		Race:                nullStringValue(character.Race),
		Background:          nullStringValue(character.Background),
		Classes:             nullStringValue(character.Classes),
		Size:                fallbackString(strings.TrimSpace(character.Size), pages.DefaultSize),
		Alignment:           fallbackString(nullStringValue(character.Alignment), pages.DefaultAlignment),
		XP:                  strconv.FormatUint(uint64(character.XP), 10),
		Languages:           fallbackString(strings.TrimSpace(character.Languages), "Common"),
		Proficiencies:       strings.TrimSpace(character.Proficiencies),
		Str:                 strconv.FormatUint(uint64(character.Str), 10),
		Dex:                 strconv.FormatUint(uint64(character.Dex), 10),
		Con:                 strconv.FormatUint(uint64(character.Con), 10),
		Int:                 strconv.FormatUint(uint64(character.Int), 10),
		Wis:                 strconv.FormatUint(uint64(character.Wis), 10),
		Cha:                 strconv.FormatUint(uint64(character.Cha), 10),
		AC:                  strconv.FormatUint(uint64(character.AC), 10),
		Speed:               fallbackString(strings.TrimSpace(character.Speed), "30 ft."),
		InitiativeBonus:     strconv.FormatInt(int64(character.InitiativeBonus), 10),
		MaxHP:               strconv.FormatUint(uint64(character.MaxHP), 10),
		CurrentHP:           strconv.FormatUint(uint64(character.CurrentHP), 10),
		TempHP:              strconv.FormatUint(uint64(character.TempHP), 10),
		SpellcastingAbility: pages.NormalizeSpellcastingAbility(string(character.SpellcastingAbility)),
		SpellBonusMisc:      strconv.FormatInt(int64(character.SpellBonusMisc), 10),
		Derived: derived,
		HitDice:            character.HitDice,
		HitDiceSpent:       strconv.FormatUint(uint64(character.HitDiceSpent), 10),
		DeathSaveSuccesses: int(character.DeathSaveSuccesses),
		DeathSaveFailures:  int(character.DeathSaveFailures),
		HeroicInspiration:  character.HeroicInspiration,
		Exhaustion:         strconv.FormatUint(uint64(character.Exhaustion), 10),
		Features:           parseFeatures(character.Features),
		PersonalityTraits: character.PersonalityTraits,
		Ideals:            character.Ideals,
		Bonds:             character.Bonds,
		Flaws:             character.Flaws,
		Age:               character.Age,
		Height:            character.Height,
		Weight:            character.Weight,
		Eyes:              character.Eyes,
		Skin:              character.Skin,
		Hair:              character.Hair,
	}
}
func characterHeader(character queries.Character) pages.CharacterHeader {
	return characterHeaderFrom(character, characterDerived(character))
}
func characterHeaderFrom(character queries.Character, derived pages.Derived) pages.CharacterHeader {
	avatar := ""
	if character.AssetID != nil {
		avatar = character.AssetID.String()
	}
	initiative := abilityModifier(character.Dex) + int(character.InitiativeBonus)
	return pages.CharacterHeader{
		Name:        character.Name,
		Subtitle:    characterSubtitle(character),
		AvatarID:    avatar,
		AC:          strconv.FormatUint(uint64(character.AC), 10),
		CurrentHP:   strconv.FormatUint(uint64(character.CurrentHP), 10),
		MaxHP:       strconv.FormatUint(uint64(character.MaxHP), 10),
		Speed:       fallbackString(strings.TrimSpace(character.Speed), "30 ft."),
		Initiative:  pages.SignedNumber(initiative),
		Proficiency: pages.SignedNumber(int(character.ProficiencyBonus)),
		Passive:     derived.PassivePerception,
	}
}
func characterSubtitle(character queries.Character) string {
	alignment := ""
	if value := nullStringValue(character.Alignment); value != "" && value != pages.DefaultAlignment {
		alignment = pages.AlignmentLabel(value)
	}
	parts := make([]string, 0, 4)
	for _, part := range []string{
		nullStringValue(character.Race),
		nullStringValue(character.Classes),
		nullStringValue(character.Background),
		alignment,
	} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " \u00b7 ")
}
func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
func fallbackString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
func parseProficiencies(raw json.RawMessage) map[string]string {
	states := map[string]string{}
	if len(raw) == 0 {
		return states
	}
	decoded := map[string]string{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return states
	}
	for key, value := range decoded {
		states[key] = pages.NormalizeProficiency(value)
	}
	return states
}
func parseStatBonuses(raw json.RawMessage) map[string]int {
	bonuses := map[string]int{}
	if len(raw) == 0 {
		return bonuses
	}
	var payload map[string]float64
	if err := json.Unmarshal(raw, &payload); err != nil {
		slog.Warn("invalid stat bonus payload; defaulting", "error", err)
		return bonuses
	}
	for key, value := range payload {
		bonuses[key] = int(value)
	}
	return bonuses
}
func parseFeatures(raw json.RawMessage) []pages.Feature {
	rows := []pages.Feature{}
	if len(raw) == 0 {
		return rows
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Warn("invalid info rows payload; defaulting", "error", err)
		return []pages.Feature{}
	}
	if rows == nil {
		rows = []pages.Feature{}
	}
	return rows
}
func nullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
func parseUint32(value string, fallback uint32) (uint32, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return fallback, err
	}
	return uint32(parsed), nil
}
func parseUint16(value string, fallback uint16) (uint16, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 16)
	if err != nil {
		return fallback, err
	}
	return uint16(parsed), nil
}
func parseUint8(value string, fallback uint8) (uint8, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 8)
	if err != nil {
		return fallback, err
	}
	return uint8(parsed), nil
}
func parseBonus(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}
func parseInt16(value string, fallback int16) (int16, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 16)
	if err != nil {
		return fallback, err
	}
	return int16(parsed), nil
}
type featurePayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
func marshalFeatureRowsPayload(r *http.Request) (json.RawMessage, error) {
	names := r.PostForm[pages.FeaturesPanel+"-name"]
	values := r.PostForm[pages.FeaturesPanel+"-value"]
	rows := make([]featurePayload, 0)
	count := len(names)
	if len(values) < count {
		count = len(values)
	}
	for i := 0; i < count; i++ {
		name := strings.TrimSpace(names[i])
		value := strings.TrimSpace(values[i])
		if name == "" && value == "" {
			continue
		}
		rows = append(rows, featurePayload{Name: name, Value: value})
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}
