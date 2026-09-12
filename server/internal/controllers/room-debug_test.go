package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/config"
)

func debugPanes(app *App) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"renderer": app.RoomDebugRendererFragment,
		"events":   app.RoomDebugEventsFragment,
		"state":    app.RoomDebugStateFragment,
	}
}
func debugRequest(handler http.HandlerFunc, pane string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/fragment/room/debug/"+pane, nil))
	return rec
}
func TestEachDebugFragmentRendersItsOwnPane(t *testing.T) {
	app := &App{Config: config.Config{Env: "development"}}
	for pane, handler := range debugPanes(app) {
		t.Run(pane, func(t *testing.T) {
			rec := debugRequest(handler, pane)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
			}
			marker := `data-debug-pane="` + pane + `"`
			if !strings.Contains(rec.Body.String(), marker) {
				t.Errorf("the %s fragment carries no %s, so nothing would paint into it", pane, marker)
			}
		})
	}
}
func TestTheDebugFragmentsAreNotServedOffADevelopmentBuild(t *testing.T) {
	app := &App{Config: config.Config{Env: "production"}}
	for pane, handler := range debugPanes(app) {
		t.Run(pane, func(t *testing.T) {
			rec := debugRequest(handler, pane)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want nothing", rec.Body.String())
			}
		})
	}
}

func TestTheServerFragmentNeedsARoomAndAHub(t *testing.T) {
	dev := config.Config{Env: "development"}
	rec := httptest.NewRecorder()
	app := &App{Config: dev}
	app.RoomDebugServerFragment(rec, httptest.NewRequest(http.MethodGet, "/fragment/room/debug/server", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d without a room, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing", rec.Body.String())
	}
}
func TestTheServerFragmentIsNotServedOffADevelopmentBuild(t *testing.T) {
	rec := httptest.NewRecorder()
	app := &App{Config: config.Config{Env: "production"}}
	app.RoomDebugServerFragment(rec, httptest.NewRequest(http.MethodGet, "/fragment/room/debug/server?room=01BX5ZZKBKACTAV9WEVGEMMVT0", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
