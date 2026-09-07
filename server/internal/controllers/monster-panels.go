package controllers

// The monster editor saves one panel at a time, and every handler below owns a
// disjoint set of columns and writes only those, through a query that names only
// those. The reason is the character sheet's and it has not changed: the parse
// helpers return their fallback on an empty string rather than an error, so a
// handler wider than its panel would answer an Identity save by writing 10 over
// every ability score and empty objects over both bonus grids -- and follow it
// with a toast saying it saved.
//
// TWO OF THE SEVEN PANELS ARE THE CHARACTER SHEET'S OWN. The bonus grids post
// the same field names because they are the same components, so
// SaveMonsterBonuses reuses bonusPanels and marshalBonusPayloads unchanged, and
// SaveMonsterAbilities reuses buildAbilitiesInput. That sharing is what makes
// "the same grid, filtered at render" true rather than aspirational.
//
// EVERY SAVE REDRAWS THE STAT BLOCK, which is the one thing this editor does
// that the sheet does not. See redrawMonster for why the row is read back rather
// than patched from what was posted.

import (
	"database/sql"
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

func (a *App) SaveMonsterIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "identity") {
		return
	}

	input, validationErrors := buildMonsterIdentityInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "identity", validationErrors)
		return
	}

	result, err := a.Queries.UpdateMonsterIdentity(ctx, queries.UpdateMonsterIdentityParams{
		Name:      input.Name,
		Size:      input.Size,
		Type:      input.Type,
		Tags:      input.Tags,
		Alignment: input.Alignment,
		ID:        monsterID,
		OwnerID:   sess.UserID,
	})
	a.finishMonsterPanel(w, r, "identity", "Identity", result, err, monsterID, sess.UserID)
}

// SaveMonsterAbilities reads the character sheet's own builder. The panel is the
// same six controls posting the same six names, and a second copy of that loop
// would be a second place for the range check to drift.
func (a *App) SaveMonsterAbilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "abilities") {
		return
	}

	input, validationErrors := buildAbilitiesInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "abilities", validationErrors)
		return
	}

	result, err := a.Queries.UpdateMonsterAbilities(ctx, queries.UpdateMonsterAbilitiesParams{
		Str:     input.Str,
		Dex:     input.Dex,
		Con:     input.Con,
		Int:     input.Int,
		Wis:     input.Wis,
		Cha:     input.Cha,
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	a.finishMonsterPanel(w, r, "abilities", "Abilities", result, err, monsterID, sess.UserID)
}

// SaveMonsterCombat owns the block's top three lines and the two counts behind
// the legendary sentence. It derives nothing, unlike the sheet's Core Stats: the
// proficiency bonus and the XP follow from cr and are worked out at render time,
// so this panel stores the rating and nothing that follows from it.
func (a *App) SaveMonsterCombat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "combat") {
		return
	}

	input, validationErrors := buildMonsterCombatInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "combat", validationErrors)
		return
	}

	result, err := a.Queries.UpdateMonsterCombat(ctx, queries.UpdateMonsterCombatParams{
		AC:                        input.AC,
		HP:                        input.HP,
		HitDice:                   input.HitDice,
		Speed:                     input.Speed,
		InitiativeBonus:           input.InitiativeBonus,
		CR:                        input.CR,
		LegendaryActionUses:       input.LegendaryActionUses,
		LegendaryActionUsesInLair: input.LegendaryActionUsesInLair,
		ID:                        monsterID,
		OwnerID:                   sess.UserID,
	})
	a.finishMonsterPanel(w, r, "combat", "Combat", result, err, monsterID, sess.UserID)
}

func (a *App) SaveMonsterDefenses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "defenses") {
		return
	}

	input, validationErrors := buildMonsterDefensesInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "defenses", validationErrors)
		return
	}

	result, err := a.Queries.UpdateMonsterDefenses(ctx, queries.UpdateMonsterDefensesParams{
		Vulnerabilities: input.Vulnerabilities,
		Resistances:     input.Resistances,
		Immunities:      input.Immunities,
		Gear:            input.Gear,
		Senses:          input.Senses,
		Languages:       input.Languages,
		ID:              monsterID,
		OwnerID:         sess.UserID,
	})
	a.finishMonsterPanel(w, r, "defenses", "Defenses", result, err, monsterID, sess.UserID)
}

// SaveMonsterBonuses serves both grids, and it is SaveCharacterBonuses with two
// statements swapped: the allowlist, the field prefixes and the payload encoder
// are shared outright, because the grids on this page ARE the grids on that one.
func (a *App) SaveMonsterBonuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}

	kind := r.PathValue("kind")
	label, ok := bonusPanels[kind]
	if !ok {
		unknownBonusKind(w, kind)
		return
	}
	if !parsePanelForm(w, r, kind) {
		return
	}

	misc, states, err := marshalBonusPayloads(r, kind)
	if err != nil {
		slog.Error("Failed to encode monster bonuses", "kind", kind, "error", err)
		htmx.ServerError(w)
		return
	}

	var result sql.Result
	switch kind {
	case "skills":
		result, err = a.Queries.UpdateMonsterSkills(ctx, queries.UpdateMonsterSkillsParams{
			Skills:             misc,
			SkillProficiencies: states,
			ID:                 monsterID,
			OwnerID:            sess.UserID,
		})
	case "saving_throws":
		result, err = a.Queries.UpdateMonsterSavingThrows(ctx, queries.UpdateMonsterSavingThrowsParams{
			SavingThrows:             misc,
			SavingThrowProficiencies: states,
			ID:                       monsterID,
			OwnerID:                  sess.UserID,
		})
	}
	a.finishMonsterPanel(w, r, kind, label, result, err, monsterID, sess.UserID)
}

func (a *App) SaveMonsterDescription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, "description") {
		return
	}

	input, validationErrors := buildMonsterDescriptionInput(r)
	if len(validationErrors) > 0 {
		renderPanelBlock(w, r, "description", validationErrors)
		return
	}

	result, err := a.Queries.UpdateMonsterDescription(ctx, queries.UpdateMonsterDescriptionParams{
		Habitat:     input.Habitat,
		Treasure:    input.Treasure,
		Description: input.Description,
		ID:          monsterID,
		OwnerID:     sess.UserID,
	})
	a.finishMonsterPanel(w, r, "description", "Description", result, err, monsterID, sess.UserID)
}

// THE PANEL BUILDERS. One per panel, each reading only the fields its panel
// renders and returning only the errors those fields can raise.

type monsterIdentityInput struct {
	Name      string
	Size      string
	Type      string
	Tags      string
	Alignment string
}

// The three selects are normalised rather than validated, which is what
// NormalizeDamageType does on the attacks row and for the same reason: the
// pickers offer nothing else, so a value off the list is a hand-built request
// rather than a mistake somebody can correct. The name is the one field here a
// person can actually get wrong.
func buildMonsterIdentityInput(r *http.Request) (monsterIdentityInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"tags", "Tags", pages.MonsterTagsLimit},
	})

	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		validationErrors = append(validationErrors, "Name is required.")
	case len([]rune(name)) > pages.MonsterNameLimit:
		validationErrors = append(validationErrors, "Name must be 128 characters or fewer.")
	}

	return monsterIdentityInput{
		Name:      name,
		Size:      pages.NormalizeSize(r.PostFormValue("size")),
		Type:      pages.NormalizeCreatureType(r.PostFormValue("type")),
		Tags:      values["tags"],
		Alignment: pages.NormalizeAlignment(r.PostFormValue("alignment")),
	}, validationErrors
}

type monsterCombatInput struct {
	AC                        uint8
	HP                        uint16
	HitDice                   string
	Speed                     string
	InitiativeBonus           int16
	CR                        string
	LegendaryActionUses       uint8
	LegendaryActionUsesInLair uint8
}

// Every bound below is the column's own, which is what lets the markup render
// the same number into a max attribute -- and what keeps MySQL's strict mode
// from turning an armour class of 256 into a 500 on a box the editor invited
// somebody to fill in.
//
// Hit points are the exception and are held at 9999 rather than the column's
// 65535, for the reason pages.MonsterHPLimit gives.
func buildMonsterCombatInput(r *http.Request) (monsterCombatInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"hit_dice", "Hit dice", pages.MonsterHitDiceLimit},
		{"speed", "Speed", pages.MonsterSpeedLimit},
	})

	ac, err := parseUint8(r.PostFormValue("ac"), 10)
	if err != nil {
		validationErrors = append(validationErrors, "Armor class must be between 0 and "+strconv.Itoa(pages.MonsterACLimit)+".")
	}

	hp, err := parseUint16(r.PostFormValue("hp"), 1)
	if err != nil || hp > pages.MonsterHPLimit {
		validationErrors = append(validationErrors, "Hit points must be between 0 and "+strconv.Itoa(pages.MonsterHPLimit)+".")
		hp = 1
	}

	initiativeBonus, err := parseInt16(r.PostFormValue("initiative_bonus"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Initiative bonus must be between -32768 and 32767.")
	}

	uses, err := parseUint8(r.PostFormValue("legendary_action_uses"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Legendary action uses must be between 0 and "+strconv.Itoa(pages.MonsterLegendaryUsesLimit)+".")
	}

	usesInLair, err := parseUint8(r.PostFormValue("legendary_action_uses_in_lair"), 0)
	if err != nil {
		validationErrors = append(validationErrors, "Legendary uses in lair must be between 0 and "+strconv.Itoa(pages.MonsterLegendaryUsesLimit)+".")
	}

	return monsterCombatInput{
		AC:      ac,
		HP:      hp,
		HitDice: values["hit_dice"],
		// The speed column has a default and the stat block falls back to it as
		// well, so an emptied box is not an error -- it is a monster whose speed
		// nobody has written down yet.
		Speed:                     values["speed"],
		InitiativeBonus:           initiativeBonus,
		CR:                        pages.NormalizeChallengeRating(r.PostFormValue("cr")),
		LegendaryActionUses:       uses,
		LegendaryActionUsesInLair: usesInLair,
	}, validationErrors
}

type monsterDefensesInput struct {
	Vulnerabilities string
	Resistances     string
	Immunities      string
	Gear            string
	Senses          string
	Languages       string
}

// Six lines of free text, and free text is the decision rather than the default:
// every one of them has a closed vocabulary behind it in the rules, and every
// one of them is printed as a sentence in the book. Structured damage types
// would buy a future VTT the ability to apply damage on its own, and would cost
// fifty-four checkboxes on this panel today.
func buildMonsterDefensesInput(r *http.Request) (monsterDefensesInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"vulnerabilities", "Vulnerabilities", pages.MonsterDefenseLimit},
		{"resistances", "Resistances", pages.MonsterDefenseLimit},
		{"immunities", "Immunities", pages.MonsterDefenseLimit},
		{"gear", "Gear", pages.MonsterDefenseLimit},
		{"senses", "Senses", pages.MonsterSensesLimit},
		{"languages", "Languages", pages.MonsterLanguagesLimit},
	})

	return monsterDefensesInput{
		Vulnerabilities: values["vulnerabilities"],
		Resistances:     values["resistances"],
		Immunities:      values["immunities"],
		Gear:            values["gear"],
		Senses:          values["senses"],
		Languages:       values["languages"],
	}, validationErrors
}

type monsterDescriptionInput struct {
	Habitat     string
	Treasure    string
	Description string
}

func buildMonsterDescriptionInput(r *http.Request) (monsterDescriptionInput, []string) {
	values, validationErrors := cappedMonsterFields(r, []cappedMonsterField{
		{"habitat", "Habitat", pages.MonsterHabitatLimit},
		{"treasure", "Treasure", pages.MonsterTreasureLimit},
	})

	// The description is TEXT and TEXT counts bytes, so this one is measured
	// with len while the two above are measured in runes. It is the same split
	// the character sheet's prose boxes make.
	description := strings.TrimSpace(r.PostFormValue("description"))
	if len(description) > pages.MonsterProseLimit {
		validationErrors = append(validationErrors, "There is too much text in the notes. Anything that long belongs in a document of its own.")
		description = ""
	}

	return monsterDescriptionInput{
		Habitat:     values["habitat"],
		Treasure:    values["treasure"],
		Description: description,
	}, validationErrors
}

// cappedMonsterField is one free-text box and the width of the column behind it.
type cappedMonsterField struct {
	Field string
	Label string
	Limit int
}

// cappedMonsterFields trims and length-checks a panel's free-text boxes in one
// loop, the way buildAppearanceInput does for the six words on the character
// sheet. THE LIMITS ARE MEASURED IN CHARACTERS because every column they guard
// is a VARCHAR and MySQL counts characters there -- len() would refuse a line of
// ninety accented letters the column would have taken.
//
// A field that fails is left out of the map rather than truncated. Nothing reads
// the map on that path: the caller returns its errors and the write never runs.
func cappedMonsterFields(r *http.Request, fields []cappedMonsterField) (map[string]string, []string) {
	values := map[string]string{}
	validationErrors := make([]string, 0)

	for _, field := range fields {
		value := strings.TrimSpace(r.PostFormValue(field.Field))
		if len([]rune(value)) > field.Limit {
			validationErrors = append(validationErrors, field.Label+" must be "+strconv.Itoa(field.Limit)+" characters or fewer.")
			continue
		}
		values[field.Field] = value
	}

	return values, validationErrors
}

// THE SHARED TAIL.

// panelMonsterID reads the {id} segment. An unparseable one is answered as a
// missing monster rather than a bad request: the only way to reach here with one
// is a stale page or a hand-edited URL, and both mean the same thing to the
// person on the other end.
func panelMonsterID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
		return ulid.ULID{}, false
	}

	return monsterID, true
}

// finishMonsterPanel is the tail every panel on this editor shares. Found-rows
// semantics make zero matched rows mean the id is not this user's rather than
// that the save changed nothing, so that is a 404 and not a silent success.
func (a *App) finishMonsterPanel(w http.ResponseWriter, r *http.Request, panel string, label string, result sql.Result, err error, monsterID, ownerID ulid.ULID) {
	if err != nil {
		slog.Error("Failed to save monster panel", "panel", panel, "error", err)
		htmx.ServerError(w)
		return
	}

	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "monster")
		return
	}

	htmx.Toast(w, label+" saved.")
	renderPanelBlock(w, r, panel, nil)

	a.redrawMonster(w, r, panel, monsterID, ownerID)
}

// redrawMonster sends the three out-of-band swaps that keep the open page honest
// after a write: the derived readouts, the bar's figure and chips, and the stat
// block itself.
//
// THE ROW IS READ BACK RATHER THAN PATCHED FROM WHAT WAS POSTED, which is what
// lets one function serve every panel and every action row. A combat save moves
// the proficiency bonus, which moves all twenty-four totals on the two grids and
// both figures on the CR line; an abilities save moves the same twenty-four plus
// the initiative and the passive perception; adding a lair action puts a second
// XP figure on a line no panel owns. No caller knows enough on its own to say
// which, and a list of which save refreshes what would be a list kept by hand.
//
// THE ACTIONS ARE READ TOO, because two numbers on the block depend on which
// sections exist rather than on any column: CR 0 is worth ten XP when the
// monster has an action, and the in-lair figure appears only when it has a lair
// action.
//
// A read that fails is logged and dropped. The save landed and the toast is
// already queued, so a stale readout that a reload fixes is a smaller thing to
// hand somebody than an error over a write that worked.
func (a *App) redrawMonster(w http.ResponseWriter, r *http.Request, panel string, monsterID, ownerID ulid.ULID) {
	ctx := r.Context()

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read the monster back", "panel", panel, "error", err)
		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{MonsterID: monsterID, OwnerID: ownerID})
	if err != nil {
		slog.Error("Failed to read the monster's actions back", "panel", panel, "error", err)
		return
	}

	derived := monsterDerived(monster, actions)
	render(w, r, pages.MonsterDerivedValues(derived))
	render(w, r, pages.MonsterBarValues(monsterHeader(monster, derived)))
	render(w, r, pages.MonsterStatBlock(monsterStatBlock(monster, actions, derived), true))
}
