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
