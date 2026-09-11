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












func TestTheSettingsEventCarriesWhatThePageIsAlreadyObeying(t *testing.T) {
	rec := httptest.NewRecorder()
	Settings(rec, `Say "hi"`, true, false, 40)

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

	
	
	
	if got := detail["pingVolume"]; got != float64(40) {
		t.Errorf("pingVolume = %#v, want the number 40", got)
	}
}





func TestASavesEventsAllSurviveEachOther(t *testing.T) {
	rec := httptest.NewRecorder()
	Theme(rec, "coffee")
	Settings(rec, "kyle", true, true, 100)
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
