package controllers

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/templ/pages"
)

func storedCharacterAnswer(size string, ac, hp, maxHP int) roomAnswer {
	now := time.Now()
	row := []struct {
		column string
		value  driver.Value
	}{
		{"id", testCharacterID.Bytes()}, {"owner_id", testOwnerID.Bytes()}, {"asset_id", nil},
		{"name", "Ilyana"}, {"level", int64(3)}, {"xp", int64(900)},
		{"race", nil}, {"background", nil}, {"alignment", nil}, {"classes", nil},
		{"size", size}, {"ac", int64(ac)}, {"max_hp", int64(maxHP)}, {"current_hp", int64(hp)},
		{"proficiency_bonus", int64(2)}, {"temp_hp", int64(0)}, {"speed", "30 ft."}, {"initiative_bonus", int64(2)},
		{"str", int64(10)}, {"dex", int64(10)}, {"con", int64(10)}, {"int", int64(10)}, {"wis", int64(10)}, {"cha", int64(10)},
		{"languages", ""}, {"proficiencies", ""},
		{"skills", []byte("{}")}, {"saving_throws", []byte("{}")}, {"features", []byte("[]")},
		{"created_at", now}, {"updated_at", now},
		{"personality_traits", ""}, {"ideals", ""}, {"bonds", ""}, {"flaws", ""},
		{"age", ""}, {"height", ""}, {"weight", ""}, {"eyes", ""}, {"skin", ""}, {"hair", ""},
		{"hit_dice", "3d8"}, {"hit_dice_spent", int64(0)},
		{"death_save_successes", int64(0)}, {"death_save_failures", int64(0)},
		{"heroic_inspiration", false}, {"exhaustion", int64(0)},
		{"skill_proficiencies", []byte("{}")}, {"saving_throw_proficiencies", []byte("{}")},
		{"spellcasting_ability", "none"}, {"spell_bonus_misc", int64(0)},
		{"copper", int64(0)}, {"silver", int64(0)}, {"electrum", int64(0)}, {"gold", int64(0)}, {"platinum", int64(0)},
		{"resources", []byte("[]")},
	}
	answer := roomAnswer{}
	for _, cell := range row {
		answer.columns = append(answer.columns, cell.column)
		answer.values = append(answer.values, cell.value)
	}
	return answer
}
func seenSave(t *testing.T, handler func(*App) http.HandlerFunc, panel string, form url.Values, stored roomAnswer) (*roomDB, *httptest.ResponseRecorder) {
	t.Helper()
	db := &roomDB{rows: 1, answers: []roomAnswer{stored, stored}}
	app := newRoomApp(db)
	rec := tableRequest(t, handler(app), http.MethodPost,
		"/characters/"+testCharacterID.String()+"/"+panel,
		map[string]string{"id": testCharacterID.String()}, form, sheetSession(false))
	return db, rec
}
func savedOver(t *testing.T, db *roomDB, query string) recordedCall {
	t.Helper()
	for _, call := range db.recorded() {
		if strings.Contains(call.query, query) {
			return call
		}
	}
	t.Fatalf("%s never ran: %v", query, db.queries())
	return recordedCall{}
}
func saveVitals(app *App) http.HandlerFunc    { return app.SaveCharacterVitals }
func saveCoreStats(app *App) http.HandlerFunc { return app.SaveCharacterCoreStats }
func saveIdentity(app *App) http.HandlerFunc  { return app.SaveCharacterIdentity }
func seen(form url.Values, field, value string) url.Values {
	return withValue(form, pages.SeenName(field), value)
}
func TestAVitalTheFormLeftAloneKeepsWhatTheTableWrote(t *testing.T) {
	form := withValue(vitalsForm(), "temp_hp", "5")
	form = seen(seen(form, "max_hp", "24"), "current_hp", "7")
	db, rec := seenSave(t, saveVitals, "vitals", form, storedCharacterAnswer("medium", 15, 3, 30))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	call := savedOver(t, db, "UpdateCharacterVitals")
	if got := writtenValue(t, call, "current_hp"); got != int64(3) {
		t.Errorf("current_hp = %v, want the 3 the table left; a stale tab put its own number back", got)
	}
	if got := writtenValue(t, call, "max_hp"); got != int64(30) {
		t.Errorf("max_hp = %v, want the 30 the table left", got)
	}
	if got := writtenValue(t, call, "temp_hp"); got != int64(5) {
		t.Errorf("temp_hp = %v, want the 5 the player typed", got)
	}
}
func TestAVitalThePlayerChangedIsWrittenOverTheTable(t *testing.T) {
	form := seen(seen(vitalsForm(), "max_hp", "24"), "current_hp", "12")
	db, _ := seenSave(t, saveVitals, "vitals", form, storedCharacterAnswer("medium", 15, 3, 30))
	call := savedOver(t, db, "UpdateCharacterVitals")
	if got := writtenValue(t, call, "current_hp"); got != int64(7) {
		t.Errorf("current_hp = %v, want the 7 the player typed", got)
	}
	if got := writtenValue(t, call, "max_hp"); got != int64(30) {
		t.Errorf("max_hp = %v, want the 30 the table left; the player did not touch it", got)
	}
}
func TestSizeAndArmourClassTheFormLeftAloneKeepWhatTheTableWrote(t *testing.T) {
	identity := seen(url.Values{"name": {"Ilyana"}, "race": {"Elf"}, "size": {"medium"}}, "size", "medium")
	db, _ := seenSave(t, saveIdentity, "identity", identity, storedCharacterAnswer("large", 18, 3, 30))
	if got := writtenValue(t, savedOver(t, db, "UpdateCharacterIdentity"), "size"); got != "large" {
		t.Errorf("size = %v, want the large the table left", got)
	}
	core := seen(url.Values{
		"xp": {"900"}, "speed": {"30 ft."}, "ac": {"15"},
		"initiative_bonus": {"2"}, "spellcasting_ability": {"none"}, "spell_bonus_misc": {"0"},
	}, "ac", "15")
	db, _ = seenSave(t, saveCoreStats, "core-stats", core, storedCharacterAnswer("large", 18, 3, 30))
	if got := writtenValue(t, savedOver(t, db, "UpdateCharacterCoreStats"), "ac"); got != int64(18) {
		t.Errorf("ac = %v, want the 18 the table left", got)
	}
}
func TestAFormThatSawNothingReadsNothingFirst(t *testing.T) {
	db, _ := seenSave(t, saveVitals, "vitals", vitalsForm(), storedCharacterAnswer("medium", 15, 3, 30))
	sent := db.queries()
	if len(sent) == 0 || !strings.Contains(sent[0], "UpdateCharacterVitals") {
		t.Fatalf("a form with no seen values did not save first: %v", sent)
	}
	if got := writtenValue(t, savedOver(t, db, "UpdateCharacterVitals"), "current_hp"); got != int64(7) {
		t.Errorf("current_hp = %v, want the 7 the form sent", got)
	}
}
func TestAGuardedSaveOfACharacterThatIsGoneWritesNothing(t *testing.T) {
	form := seen(seen(vitalsForm(), "max_hp", "24"), "current_hp", "7")
	db, rec := seenSave(t, saveVitals, "vitals", form, roomAnswer{})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	for _, query := range db.queries() {
		if strings.Contains(query, "UpdateCharacterVitals") {
			t.Errorf("a character that is gone was still written: %v", db.queries())
		}
	}
}
func TestASaveTellsTheFormWhatItNowShows(t *testing.T) {
	form := seen(seen(vitalsForm(), "max_hp", "24"), "current_hp", "12")
	form.Set("current_hp", "9")
	form.Set("max_hp", "24")
	_, rec := seenSave(t, saveVitals, "vitals", form, storedCharacterAnswer("medium", 15, 3, 30))
	body := rec.Body.String()
	for field, want := range map[string]string{"current_hp": "9", "max_hp": "24"} {
		tag := seenTag(t, body, field)
		if !strings.Contains(tag, `value="`+want+`"`) {
			t.Errorf("%s: the form is told it shows something other than %s\n%s", field, want, tag)
		}
		if !strings.Contains(tag, `hx-swap-oob="true"`) {
			t.Errorf("%s: the seen value does not swap into the form\n%s", field, tag)
		}
	}
}
func seenTag(t *testing.T, body, field string) string {
	t.Helper()
	return tagByID(t, body, "seen-"+field)
}
func tagByID(t *testing.T, body, id string) string {
	t.Helper()
	at := strings.Index(body, `id="`+id+`"`)
	if at < 0 {
		t.Fatalf("nothing carries id %s in\n%s", id, body)
	}
	open := strings.LastIndex(body[:at], "<")
	end := strings.Index(body[at:], ">")
	return body[open : at+end+1]
}
