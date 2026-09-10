package htmx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestToastSurvivesQuotesInTheMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	Toast(rec, `Say "hi" has been deleted.`)

	var events map[string]string
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}
	if got := events["flash:toast"]; got != `Say "hi" has been deleted.` {
		t.Errorf("flash:toast = %q", got)
	}
}

func TestTriggerMergesWithAnExistingHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	Toast(rec, "saved")
	Error(rec, "Heads up", "but also this", 400)

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}
	if _, ok := events["flash:toast"]; !ok {
		t.Error("toast was dropped by the later alert")
	}
	if _, ok := events["alert"]; !ok {
		t.Error("alert missing")
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// THE SETTINGS EVENT IS WHAT REPLACED AN OUT-OF-BAND SWAP, so the shape of it
// is the contract with two listeners that cannot see this file:
// public/js/account-name.js reads the name, and server/js/room/main.ts reads the
// two booleans. A renamed field is a setting that silently stops applying.
//
// THE BOOLEANS ARE BOOLEANS AND NOT STRINGS, which is the one thing a
// map[string]any could get wrong here: the room reads `detail.showBlood !==
// false`, and the string "false" is not false.
func TestTheSettingsEventCarriesWhatThePageIsAlreadyObeying(t *testing.T) {
	rec := httptest.NewRecorder()
	Settings(rec, `Say "hi"`, true, false)

	var events map[string]map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}

	detail, ok := events["settings:change"]
	if !ok {
		t.Fatalf("no settings:change in %v", events)
	}
	if got := detail["name"]; got != `Say "hi"` {
		t.Errorf("name = %#v", got)
	}
	if got := detail["followTurn"]; got != true {
		t.Errorf("followTurn = %#v, want the boolean true", got)
	}
	if got := detail["showBlood"]; got != false {
		t.Errorf("showBlood = %#v, want the boolean false", got)
	}
}

// AND IT MERGES WITH THE REST OF THE REPLY. A save raises four events -- the
// theme, this, the dismissal and the toast -- and every one of them goes through
// the same header, so any of them clobbering another would be a setting that
// applied only when it was saved alone.
func TestASavesEventsAllSurviveEachOther(t *testing.T) {
	rec := httptest.NewRecorder()
	Theme(rec, "coffee")
	Settings(rec, "kyle", true, true)
	CloseModal(rec)
	Toast(rec, "Settings saved.")

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}

	for _, name := range []string{"theme:change", "settings:change", "modal:close", "flash:toast"} {
		if _, ok := events[name]; !ok {
			t.Errorf("%s was dropped by the events queued after it", name)
		}
	}
}
