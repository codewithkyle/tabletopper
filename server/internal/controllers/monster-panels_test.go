package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)



func monsterPanelForm(panel string) url.Values {
	switch panel {
	case "identity":
		return url.Values{
			"name":      {"Goblin Boss"},
			"size":      {"small"},
			"type":      {"humanoid"},
			"tags":      {"Goblinoid"},
			"alignment": {"chaotic neutral"},
		}
	case "abilities":
		return url.Values{"str": {"10"}, "dex": {"14"}, "con": {"10"}, "int": {"10"}, "wis": {"8"}, "cha": {"10"}}
	case "combat":
		return url.Values{
			"ac":                            {"17"},
			"hp":                            {"21"},
			"hit_dice":                      {"6d6"},
			"speed":                         {"30 ft."},
			"initiative_bonus":              {"0"},
			"cr":                            {"1"},
			"legendary_action_uses":         {"0"},
			"legendary_action_uses_in_lair": {"0"},
		}
	case "defenses":
		return url.Values{
			"vulnerabilities": {"Fire"},
			"resistances":     {"Cold"},
			"immunities":      {"Poison; Poisoned"},
			"gear":            {"Chain Shirt"},
			"senses":          {"Darkvision 60 ft."},
			"languages":       {"Common, Goblin"},
		}
	case "description":
		return url.Values{
			"habitat":     {"Forest, Urban"},
			"treasure":    {"Individual"},
			"description": {"Bullies the smaller goblins and runs when it is losing."},
		}
	}

	return url.Values{}
}

func monsterPanelHandler(panel string) func(*App) http.HandlerFunc {
	switch panel {
	case "identity":
		return func(a *App) http.HandlerFunc { return a.SaveMonsterIdentity }
	case "abilities":
		return func(a *App) http.HandlerFunc { return a.SaveMonsterAbilities }
	case "combat":
		return func(a *App) http.HandlerFunc { return a.SaveMonsterCombat }
	case "defenses":
		return func(a *App) http.HandlerFunc { return a.SaveMonsterDefenses }
	}

	return func(a *App) http.HandlerFunc { return a.SaveMonsterDescription }
}




func TestMonsterPanelsWriteOnlyTheirOwnColumns(t *testing.T) {
	for _, c := range []struct {
		name       string
		handler    func(*App) http.HandlerFunc
		form       url.Values
		pathValues map[string]string
		want       []string
	}{
		{
			name:    "identity",
			handler: monsterPanelHandler("identity"),
			form:    monsterPanelForm("identity"),
			want:    []string{"alignment", "name", "size", "tags", "type"},
		},
		{
			name:    "abilities",
			handler: monsterPanelHandler("abilities"),
			form:    monsterPanelForm("abilities"),
			want:    []string{"cha", "con", "dex", "int", "str", "wis"},
		},
		{
			name:    "combat",
			handler: monsterPanelHandler("combat"),
			form:    monsterPanelForm("combat"),
			want:    []string{"ac", "cr", "hit_dice", "hp", "initiative_bonus", "legendary_action_uses", "legendary_action_uses_in_lair", "speed"},
		},
		{
			name:    "defenses",
			handler: monsterPanelHandler("defenses"),
			form:    monsterPanelForm("defenses"),
			want:    []string{"gear", "immunities", "languages", "resistances", "senses", "vulnerabilities"},
		},
		{
			name:    "description",
			handler: monsterPanelHandler("description"),
			form:    monsterPanelForm("description"),
			want:    []string{"description", "habitat", "treasure"},
		},
		{
			name:       "skills",
			handler:    func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses },
			form:       url.Values{"skills-stealth-misc": {"2"}, "skills-stealth-proficiency": {"proficient"}},
			pathValues: map[string]string{"kind": "skills"},
			want:       []string{"skill_proficiencies", "skills"},
		},
		{
			name:       "saving throws",
			handler:    func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses },
			form:       url.Values{"saving_throws-dex-misc": {"1"}, "saving_throws-dex-proficiency": {"proficient"}},
			pathValues: map[string]string{"kind": "saving_throws"},
			want:       []string{"saving_throw_proficiencies", "saving_throws"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := panelPost(t, db, c.handler(app), c.form, monsterPathValues(c.pathValues))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}

			call := db.only(t)
			if !strings.Contains(call.query, "UPDATE monsters") {
				t.Errorf("a monster panel wrote something other than monsters:\n%s", call.query)
			}
			got := sortedColumns(t, call.query)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("columns written = %v, want %v", got, c.want)
			}
			if call.args[len(call.args)-1] != testOwnerID {
				t.Errorf("owner scoping missing; last arg = %v", call.args[len(call.args)-1])
			}
		})
	}
}

func monsterPathValues(extra map[string]string) map[string]string {
	pathValues := map[string]string{"id": testMonsterID.String()}
	for key, value := range extra {
		pathValues[key] = value
	}

	return pathValues
}



var unownedMonsterColumns = map[string]bool{
	"id":         true,
	"owner_id":   true,
	"asset_id":   true,
	"created_at": true,
	"updated_at": true,
}









func TestMonsterPanelsCoverEveryEditableColumn(t *testing.T) {
	covered := map[string]bool{}

	for _, panel := range []struct {
		name       string
		handler    func(*App) http.HandlerFunc
		pathValues map[string]string
	}{
		{name: "identity", handler: monsterPanelHandler("identity")},
		{name: "abilities", handler: monsterPanelHandler("abilities")},
		{name: "combat", handler: monsterPanelHandler("combat")},
		{name: "defenses", handler: monsterPanelHandler("defenses")},
		{name: "description", handler: monsterPanelHandler("description")},
		{name: "skills", handler: func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses }, pathValues: map[string]string{"kind": "skills"}},
		{name: "saving throws", handler: func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses }, pathValues: map[string]string{"kind": "saving_throws"}},
	} {
		app, db := newPanelApp(1)

		
		panelPost(t, db, panel.handler(app), url.Values{"name": {"Goblin"}}, monsterPathValues(panel.pathValues))
		for _, column := range sortedColumns(t, db.only(t).query) {
			if covered[column] {
				t.Errorf("column %q is written by two panels", column)
			}
			covered[column] = true
		}
	}

	for _, column := range tableColumns(t, "monsters") {
		if unownedMonsterColumns[column] {
			if covered[column] {
				t.Errorf("column %q is not meant to be editable and a panel writes it", column)
			}
			continue
		}
		if !covered[column] {
			t.Errorf("column %q is editable and no panel writes it", column)
		}
	}
}




func TestMonsterPanelValidationFailsBeforeTheWrite(t *testing.T) {
	form := monsterPanelForm("identity")
	form.Set("name", "   ")

	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.SaveMonsterIdentity, form, monsterPathValues(nil))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Errorf("the rejected panel wrote anyway: %v", db.calls)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Name is required.") {
		t.Errorf("body missing the message: %s", body)
	}
	if !strings.Contains(body, `id="errors-identity"`) {
		t.Errorf("body is not the panel's error block: %s", body)
	}
}




func TestOverlongMonsterFieldsAreRejectedNotTruncated(t *testing.T) {
	for _, c := range []struct {
		panel string
		field string
		limit int
		want  string
	}{
		{"identity", "name", pages.MonsterNameLimit, "Name must be 128 characters or fewer."},
		{"identity", "tags", pages.MonsterTagsLimit, "Tags must be 128 characters or fewer."},
		{"combat", "hit_dice", pages.MonsterHitDiceLimit, "Hit dice must be 64 characters or fewer."},
		{"combat", "speed", pages.MonsterSpeedLimit, "Speed must be 128 characters or fewer."},
		{"defenses", "vulnerabilities", pages.MonsterDefenseLimit, "Vulnerabilities must be 512 characters or fewer."},
		{"defenses", "resistances", pages.MonsterDefenseLimit, "Resistances must be 512 characters or fewer."},
		{"defenses", "immunities", pages.MonsterDefenseLimit, "Immunities must be 512 characters or fewer."},
		{"defenses", "gear", pages.MonsterDefenseLimit, "Gear must be 512 characters or fewer."},
		{"defenses", "senses", pages.MonsterSensesLimit, "Senses must be 255 characters or fewer."},
		{"defenses", "languages", pages.MonsterLanguagesLimit, "Languages must be 255 characters or fewer."},
		{"description", "habitat", pages.MonsterHabitatLimit, "Habitat must be 255 characters or fewer."},
		{"description", "treasure", pages.MonsterTreasureLimit, "Treasure must be 64 characters or fewer."},
	} {
		t.Run(c.panel+"/"+c.field, func(t *testing.T) {
			form := monsterPanelForm(c.panel)
			form.Set(c.field, strings.Repeat("a", c.limit+1))

			app, db := newPanelApp(1)
			rec := panelPost(t, db, monsterPanelHandler(c.panel)(app), form, monsterPathValues(nil))

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Error("the overlong value was sent to the column anyway")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body = %q, want it to carry %q", body, c.want)
			}

			
			
			
			form.Set(c.field, strings.Repeat("é", c.limit))

			app, db = newPanelApp(1)
			rec = panelPost(t, db, monsterPanelHandler(c.panel)(app), form, monsterPathValues(nil))
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200: %d accented letters fit the column", rec.Code, c.limit)
			}
			if len(db.calls) != 1 {
				t.Errorf("ran %d statements, want 1", len(db.calls))
			}
		})
	}
}



func TestOverlongMonsterNotesAreMeasuredInBytes(t *testing.T) {
	form := monsterPanelForm("description")
	form.Set("description", strings.Repeat("a", pages.MonsterProseLimit+1))

	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.SaveMonsterDescription, form, monsterPathValues(nil))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Error("the overlong notes were sent to the column anyway")
	}

	
	form.Set("description", strings.Repeat("é", pages.MonsterProseLimit))

	app, db = newPanelApp(1)
	rec = panelPost(t, db, app.SaveMonsterDescription, form, monsterPathValues(nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422: %d accented letters is twice the byte cap", rec.Code, pages.MonsterProseLimit)
	}
	if len(db.calls) != 0 {
		t.Error("a value past the byte cap reached the column")
	}
}





func TestMonsterSelectsNormaliseAnythingNotOnTheList(t *testing.T) {
	identity := monsterPanelForm("identity")
	identity.Set("size", "enormous")
	identity.Set("type", "wyrm")
	identity.Set("alignment", "sideways")

	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.SaveMonsterIdentity, identity, monsterPathValues(nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: an unknown choice is corrected, not refused", rec.Code)
	}

	call := db.only(t)
	for _, c := range []struct{ column, want string }{
		{"size", pages.DefaultSize},
		{"type", pages.DefaultCreatureType},
		{"alignment", pages.DefaultAlignment},
	} {
		if got := writtenValue(t, call, c.column); got != c.want {
			t.Errorf("%s = %q, want %q", c.column, got, c.want)
		}
	}

	combat := monsterPanelForm("combat")
	combat.Set("cr", "31")

	app, db = newPanelApp(1)
	rec = panelPost(t, db, app.SaveMonsterCombat, combat, monsterPathValues(nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := writtenValue(t, db.only(t), "cr"); got != pages.DefaultChallengeRating {
		t.Errorf("cr = %q, want %q", got, pages.DefaultChallengeRating)
	}
}




func TestMonsterNumbersOutsideTheirColumnsAreRejected(t *testing.T) {
	for _, c := range []struct {
		field string
		value string
		want  string
	}{
		{"ac", "256", "Armor class must be between 0 and 255."},
		{"hp", "10000", "Hit points must be between 0 and 9999."},
		{"legendary_action_uses", "256", "Legendary action uses must be between 0 and 255."},
	} {
		t.Run(c.field, func(t *testing.T) {
			form := monsterPanelForm("combat")
			form.Set(c.field, c.value)

			app, db := newPanelApp(1)
			rec := panelPost(t, db, app.SaveMonsterCombat, form, monsterPathValues(nil))

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Error("the out-of-range value was sent to the column anyway")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body = %q, want it to carry %q", body, c.want)
			}
		})
	}
}












func TestASaveRedrawsTheStatBlock(t *testing.T) {
	for _, c := range []struct {
		name       string
		handler    func(*App) http.HandlerFunc
		pathValues map[string]string
	}{
		{name: "identity", handler: monsterPanelHandler("identity")},
		{name: "abilities", handler: monsterPanelHandler("abilities")},
		{name: "combat", handler: monsterPanelHandler("combat")},
		{name: "defenses", handler: monsterPanelHandler("defenses")},
		{name: "description", handler: monsterPanelHandler("description")},
		{name: "skills", handler: func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses }, pathValues: map[string]string{"kind": "skills"}},
		{name: "saving throws", handler: func(a *App) http.HandlerFunc { return a.SaveMonsterBonuses }, pathValues: map[string]string{"kind": "saving_throws"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			panelPost(t, db, c.handler(app), url.Values{"name": {"Goblin"}}, monsterPathValues(c.pathValues))

			if len(db.reads) == 0 {
				t.Fatal("saved without reading the monster back, so the block it was saved from is now stale")
			}

			read := db.reads[0]
			if !strings.Contains(read.query, "FROM monsters") {
				t.Errorf("the redraw read something other than the monster:\n%s", read.query)
			}
			if len(read.args) != 2 || read.args[0] != testMonsterID || read.args[1] != testOwnerID {
				t.Errorf("the redraw is not scoped to this user's monster: %v", read.args)
			}
		})
	}
}




func TestStatBlockFragmentRejectsABadID(t *testing.T) {
	for _, query := range []string{"", "monster=", "monster=not-a-ulid", "monster=" + testMonsterID.String() + "x"} {
		app, db := newPanelApp(1)

		r := httptest.NewRequest(http.MethodGet, "/fragment/monster/stat-block?"+query, nil)
		r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
		rec := httptest.NewRecorder()
		app.MonsterStatBlockFragment(rec, r)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%q: status = %d, want 404", query, rec.Code)
		}
		if body := rec.Body.String(); body != "" {
			t.Errorf("%q: body = %q, want empty", query, body)
		}
		if len(db.calls) != 0 || len(db.reads) != 0 {
			t.Errorf("%q: a bad id reached the database", query)
		}
	}
}



func TestStatBlockFragmentReadsTheOwnersMonster(t *testing.T) {
	app, db := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/fragment/monster/stat-block?monster="+testMonsterID.String(), nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	app.MonsterStatBlockFragment(httptest.NewRecorder(), r)

	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	read := db.reads[0]
	if !strings.Contains(read.query, "FROM monsters") {
		t.Errorf("read something other than the monster:\n%s", read.query)
	}
	if len(read.args) != 2 || read.args[0] != testMonsterID || read.args[1].(ulid.ULID) != testOwnerID {
		t.Errorf("the read is not scoped to this user's monster: %v", read.args)
	}
}
