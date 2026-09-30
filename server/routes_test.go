package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tabletopper/internal/controllers"
	"tabletopper/internal/middleware"
)

func TestRoutesRegisterWithoutConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()
	routes(&controllers.App{}, middleware.Auth{})
}

func TestPanelRoutesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	item := "01BX5ZZKBKACTAV9WEVGEMMVS0"
	asset := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	token := "yA1rMcJ4TkK9wQ2sVbNpXg"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/characters/" + id + "/avatar", "POST /characters/{id}/avatar"},
		{http.MethodPost, "/characters/" + id + "/identity", "POST /characters/{id}/identity"},
		{http.MethodPost, "/characters/" + id + "/core-stats", "POST /characters/{id}/core-stats"},
		{http.MethodPost, "/characters/" + id + "/bonuses/skills", "POST /characters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/characters/" + id + "/features", "POST /characters/{id}/features"},
		{http.MethodGet, "/characters/" + id + "/edit", "GET /characters/{id}/edit"},
		{http.MethodGet, "/characters/" + id + "/edit/spells", "GET /characters/{id}/edit/spells"},
		{http.MethodGet, "/characters/" + id + "/edit/spells/0", "GET /characters/{id}/edit/spells/{level}"},
		{http.MethodGet, "/characters/" + id + "/edit/spells/3", "GET /characters/{id}/edit/spells/{level}"},
		{http.MethodGet, "/characters/" + id + "/edit/inventory", "GET /characters/{id}/edit/inventory"},
		{http.MethodPost, "/characters/" + id + "/inventory", "POST /characters/{id}/inventory"},
		{http.MethodPost, "/characters/" + id + "/inventory/" + item, "POST /characters/{id}/inventory/{itemId}"},
		{http.MethodDelete, "/characters/" + id + "/inventory/" + item, "DELETE /characters/{id}/inventory/{itemId}"},
		{http.MethodPost, "/characters/" + id + "/spells/slots/3", "POST /characters/{id}/spells/slots/{level}"},
		{http.MethodPost, "/characters/" + id + "/spells/3", "POST /characters/{id}/spells/{level}"},
		{http.MethodPost, "/characters/" + id + "/spells/3/" + item, "POST /characters/{id}/spells/{level}/{spellId}"},
		{http.MethodDelete, "/characters/" + id + "/spells/3/" + item, "DELETE /characters/{id}/spells/{level}/{spellId}"},
		{http.MethodGet, "/characters/" + id + "/edit/journal", "GET /characters/{id}/edit/journal"},
		{http.MethodGet, "/characters/" + id + "/edit/journal/" + item, "GET /characters/{id}/edit/journal/{entryId}"},
		{http.MethodPost, "/characters/" + id + "/journal", "POST /characters/{id}/journal"},
		{http.MethodPost, "/characters/" + id + "/journal/" + item, "POST /characters/{id}/journal/{entryId}"},
		{http.MethodDelete, "/characters/" + id + "/journal/" + item, "DELETE /characters/{id}/journal/{entryId}"},
		{http.MethodPost, "/characters/" + id + "/share", "POST /characters/{id}/share"},
		{http.MethodDelete, "/characters/" + id + "/share", "DELETE /characters/{id}/share"},
		{http.MethodGet, "/characters/" + id + "/export.md", "GET /characters/{id}/export.md"},
		{http.MethodPost, "/characters/" + id + "/journal/" + item + "/share", "POST /characters/{id}/journal/{entryId}/share"},
		{http.MethodDelete, "/characters/" + id + "/journal/" + item + "/share", "DELETE /characters/{id}/journal/{entryId}/share"},
		{http.MethodPost, "/characters/" + id + "/journal/" + item + "/images", "POST /characters/{id}/journal/{entryId}/images"},
		{http.MethodGet, "/characters/" + id + "/journal/" + item + "/images/" + asset, "GET /characters/{id}/journal/{entryId}/images/{assetId}"},
		{http.MethodGet, "/characters/" + id + "/journal/" + item + "/images", "/"},
		{http.MethodGet, "/monsters", "GET /monsters"},
		{http.MethodPost, "/monsters", "POST /monsters"},
		{http.MethodDelete, "/monsters/" + id, "DELETE /monsters/{id}"},
		{http.MethodGet, "/monsters/" + id + "/edit", "GET /monsters/{id}/edit"},
		{http.MethodPost, "/monsters/" + id + "/identity", "POST /monsters/{id}/identity"},
		{http.MethodPost, "/monsters/" + id + "/abilities", "POST /monsters/{id}/abilities"},
		{http.MethodPost, "/monsters/" + id + "/combat", "POST /monsters/{id}/combat"},
		{http.MethodPost, "/monsters/" + id + "/defenses", "POST /monsters/{id}/defenses"},
		{http.MethodPost, "/monsters/" + id + "/description", "POST /monsters/{id}/description"},
		{http.MethodPost, "/monsters/" + id + "/bonuses/skills", "POST /monsters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/bonuses/saving_throws", "POST /monsters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/actions/trait", "POST /monsters/{id}/actions/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/actions/legendary_action", "POST /monsters/{id}/actions/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/actions/trait/" + item, "POST /monsters/{id}/actions/{kind}/{actionId}"},
		{http.MethodDelete, "/monsters/" + id + "/actions/trait/" + item, "DELETE /monsters/{id}/actions/{kind}/{actionId}"},
		{http.MethodPost, "/monsters/" + id + "/actions/mythic_action", "POST /monsters/{id}/actions/{kind}"},
		{http.MethodGet, "/monsters/" + id + "/actions/trait", "/"},
		{http.MethodPost, "/monsters/" + id + "/image", "POST /monsters/{id}/image"},
		{http.MethodPost, "/monsters/" + id + "/share", "POST /monsters/{id}/share"},
		{http.MethodDelete, "/monsters/" + id + "/share", "DELETE /monsters/{id}/share"},
		{http.MethodGet, "/monsters/" + id + "/export.md", "GET /monsters/{id}/export.md"},
		{http.MethodGet, "/monsters/" + id + "/export", "/"},
		{http.MethodPost, "/monsters/" + id + "/export.md", "/"},
		{http.MethodGet, "/monsters/new", "/"},
		{http.MethodGet, "/fragment/character/new", "GET /fragment/character/new"},
		{http.MethodGet, "/fragment/monster/new", "GET /fragment/monster/new"},
		{http.MethodGet, "/fragment/monster/share?monster=" + id, "GET /fragment/monster/share"},
		{http.MethodGet, "/fragment/monster/list", "GET /fragment/monster/list"},
		{http.MethodGet, "/fragment/monster/list?q=goblin", "GET /fragment/monster/list"},
		{http.MethodPost, "/fragment/monster/list", "/fragment/"},
		{http.MethodGet, "/fragment/monster/stat-block?monster=" + id, "GET /fragment/monster/stat-block"},
		{http.MethodPost, "/fragment/monster/stat-block", "/fragment/"},
		{http.MethodGet, "/fragment/character/sheet?room=" + id, "GET /fragment/character/sheet"},
		{http.MethodGet, "/fragment/character/sheet?room=" + id + "&section=main", "GET /fragment/character/sheet"},
		{http.MethodPost, "/fragment/character/sheet", "/fragment/"},
		{http.MethodGet, "/fragment/character/feature-row", "GET /fragment/character/feature-row"},
		{http.MethodGet, "/fragment/character/journal-link", "GET /fragment/character/journal-link"},
		{http.MethodGet, "/fragment/character/journal-entries", "GET /fragment/character/journal-entries"},
		{http.MethodGet, "/fragment/character/journal-entries?character=" + id + "&q=hag", "GET /fragment/character/journal-entries"},
		{http.MethodPost, "/fragment/character/journal-entries", "/fragment/"},
		{http.MethodPost, "/characters/" + id + "/rows", "/"},
		{http.MethodPost, "/characters/" + id, "/"},
		{http.MethodGet, "/characters/new", "/"},
		{http.MethodPost, "/characters/" + id + "/spells", "/"},
		{http.MethodGet, "/characters/" + id + "/inventory", "/"},
		{http.MethodDelete, "/characters/" + id + "/inventory", "/"},
		{http.MethodPost, "/characters/" + id + "/rows/features", "/"},
		{http.MethodGet, "/fragment/character/info-row", "/fragment/"},
		{http.MethodGet, "/fragment/character/spell-card", "/fragment/"},
		{http.MethodGet, "/share/" + token, "GET /share/{token}"},
		{http.MethodPost, "/share/" + token, "POST /share/{token}"},
		{http.MethodPost, "/share/" + token + "/import", "POST /share/{token}/import"},
		{http.MethodGet, "/share/" + token + "/portrait", "GET /share/{token}/portrait"},
		{http.MethodGet, "/share/" + token + "/export.md", "GET /share/{token}/export.md"},
		{http.MethodGet, "/share/" + token + "/images/" + asset, "GET /share/{token}/images/{assetId}"},
		{http.MethodGet, "/share/" + token + "/import", "/"},
		{http.MethodPost, "/share/" + token + "/portrait", "/"},
		{http.MethodGet, "/share/" + token + "/avatar", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

func TestMapRoutesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)
	id := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	gen := "01BX5ZZKBKACTAV9WEVGEMMVS3"
	tiles := "/assets/maps/" + id + "/tiles"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/assets/maps", "POST /assets/maps"},
		{http.MethodPost, "/assets/maps/" + id, "POST /assets/maps/{id}"},
		{http.MethodDelete, "/assets/maps/" + id, "DELETE /assets/maps/{id}"},
		{http.MethodPatch, "/assets/maps/" + id + "/name", "PATCH /assets/maps/{id}/name"},
		{http.MethodPost, tiles, "POST /assets/maps/{id}/tiles"},
		{http.MethodGet, tiles + "/" + gen + "/3/2_1.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/" + gen + "/0/23_17.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/" + gen + "/3/2_1", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/not-a-ulid/3/2_1.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/" + gen + "/3/", "/"},
		{http.MethodGet, tiles + "/" + gen + "/3/z/2_1.webp", "/"},
		{http.MethodPost, tiles + "/" + gen + "/3/2_1.webp", "/"},
		{http.MethodGet, tiles, "/"},
		{http.MethodGet, tiles + "/" + gen, "/"},
		{http.MethodGet, "/fragment/assets/maps/" + id + "/card", "GET /fragment/assets/maps/{id}/card"},
		{http.MethodPost, "/fragment/assets/maps/" + id + "/card", "/fragment/"},
		{http.MethodGet, "/fragment/assets/maps/" + id, "/fragment/"},
		{http.MethodGet, "/assets/maps/" + id + "/card", "/"},
		{http.MethodGet, "/fragment/assets/list", "GET /fragment/assets/list"},
		{http.MethodGet, "/fragment/assets/list?kind=maps&q=keep", "GET /fragment/assets/list"},
		{http.MethodPost, "/fragment/assets/list", "/fragment/"},
		{http.MethodDelete, "/fragment/assets/list", "/fragment/"},
		{http.MethodGet, "/fragment/assets/list/maps", "/fragment/"},
		{http.MethodGet, "/assets/list", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

func TestAssetKindPagesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)
	asset := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodGet, "/assets", "GET /assets"},
		{http.MethodGet, "/assets/maps", "GET /assets/maps"},
		{http.MethodGet, "/assets/tokens", "GET /assets/tokens"},
		{http.MethodGet, "/assets/avatars", "GET /assets/avatars"},
		{http.MethodGet, "/assets/music", "GET /assets/music"},
		{http.MethodGet, "/assets/images/" + asset, "GET /assets/images/{id}"},
		{http.MethodGet, "/assets/images/" + asset + "/preview", "GET /assets/images/{id}/preview"},
		{http.MethodGet, "/assets/images", "/"},
		{http.MethodPost, "/assets/maps", "POST /assets/maps"},
		{http.MethodPost, "/assets/tokens", "POST /assets/tokens"},
		{http.MethodPost, "/assets/tokens/" + asset, "POST /assets/tokens/{id}"},
		{http.MethodPatch, "/assets/tokens/" + asset + "/name", "PATCH /assets/tokens/{id}/name"},
		{http.MethodDelete, "/assets/tokens/" + asset, "DELETE /assets/tokens/{id}"},
		{http.MethodPost, "/assets/avatars", "POST /assets/avatars"},
		{http.MethodPost, "/assets/avatars/" + asset, "POST /assets/avatars/{id}"},
		{http.MethodPatch, "/assets/avatars/" + asset + "/name", "PATCH /assets/avatars/{id}/name"},
		{http.MethodDelete, "/assets/avatars/" + asset, "DELETE /assets/avatars/{id}"},
		{http.MethodDelete, "/assets/tokens", "/"},
		{http.MethodPatch, "/assets/tokens/" + asset, "/"},
		{http.MethodGet, "/assets/tokens/" + asset, "/"},
		{http.MethodGet, "/assets/avatars/" + asset, "/"},
		{http.MethodPost, "/assets/music", "POST /assets/music"},
		{http.MethodPost, "/assets/music/" + asset + "/confirm", "POST /assets/music/{id}/confirm"},
		{http.MethodPatch, "/assets/music/" + asset + "/name", "PATCH /assets/music/{id}/name"},
		{http.MethodDelete, "/assets/music/" + asset, "DELETE /assets/music/{id}"},
		{http.MethodGet, "/assets/music/" + asset + "/audio", "GET /assets/music/{id}/audio"},
		{http.MethodPost, "/assets/music/" + asset, "/"},
		{http.MethodGet, "/assets/music/" + asset + "/confirm", "/"},
		{http.MethodPost, "/assets/music/" + asset + "/audio", "/"},
		{http.MethodGet, "/assets/music/" + asset, "/"},
		{http.MethodGet, "/assets/handouts", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

func TestRoomRoutesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)
	id := "01BX5ZZKBKACTAV9WEVGEMMVT0"
	code := "AB2C"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodGet, "/rooms", "GET /rooms"},
		{http.MethodPost, "/rooms", "POST /rooms"},
		{http.MethodGet, "/rooms/join", "GET /rooms/join"},
		{http.MethodPost, "/rooms/join", "POST /rooms/join"},
		{http.MethodGet, "/rooms/join/" + code, "GET /rooms/join/{code}"},
		{http.MethodGet, "/rooms/" + id, "GET /rooms/{id}"},
		{http.MethodDelete, "/rooms/" + id, "DELETE /rooms/{id}"},
		{http.MethodPost, "/rooms/" + id + "/lock", "POST /rooms/{id}/lock"},
		{http.MethodPost, "/rooms/" + id + "/unlock", "POST /rooms/{id}/unlock"},
		{http.MethodPost, "/rooms/" + id + "/close", "POST /rooms/{id}/close"},
		{http.MethodPost, "/rooms/" + id + "/open", "POST /rooms/{id}/open"},
		{http.MethodPost, "/rooms/" + id + "/leave", "POST /rooms/{id}/leave"},
		{http.MethodPost, "/rooms/" + id + "/players/" + id + "/kick", "POST /rooms/{id}/players/{player}/kick"},
		{http.MethodGet, "/rooms/" + id + "/players", "/"},
		{http.MethodGet, "/rooms/" + id + "/players/" + id, "/"},
		{http.MethodDelete, "/rooms/" + id + "/players/" + id, "/"},
		{http.MethodPost, "/rooms/" + id + "/layers", "POST /rooms/{id}/layers"},
		{http.MethodDelete, "/rooms/" + id + "/layers/" + id, "DELETE /rooms/{id}/layers/{layer}"},
		{http.MethodPatch, "/rooms/" + id + "/layers/" + id + "/name", "PATCH /rooms/{id}/layers/{layer}/name"},
		{http.MethodPost, "/rooms/" + id + "/layers/" + id + "/move", "POST /rooms/{id}/layers/{layer}/move"},
		{http.MethodPost, "/rooms/" + id + "/layers/" + id + "/activate", "POST /rooms/{id}/layers/{layer}/activate"},
		{http.MethodPost, "/rooms/" + id + "/layers/" + id + "/map", "POST /rooms/{id}/layers/{layer}/map"},
		{http.MethodDelete, "/rooms/" + id + "/layers/" + id + "/map", "DELETE /rooms/{id}/layers/{layer}/map"},
		{http.MethodPost, "/rooms/" + id + "/grid", "POST /rooms/{id}/grid"},
		{http.MethodGet, "/rooms/" + id + "/layers", "/"},
		{http.MethodGet, "/rooms/" + id + "/layers/" + id, "/"},
		{http.MethodGet, "/rooms/" + id + "/grid", "/"},
		{http.MethodPost, "/rooms/" + id + "/layers/" + id + "/grid", "/"},
		{http.MethodGet, "/socket/room/" + id, "GET /socket/room/{id}"},
		{http.MethodGet, "/rooms/" + id + "/socket", "/"},
		{http.MethodPost, "/socket/room/" + id, "/"},
		{http.MethodGet, "/socket/room/" + id + "/frames", "/"},
		{http.MethodPost, "/rooms/" + id + "/join", "/"},
		{http.MethodPost, "/rooms/join/" + code, "/"},
		{http.MethodGet, "/rooms/new", "GET /rooms/{id}"},
		{http.MethodPost, "/rooms/" + id, "/"},
		{http.MethodPatch, "/rooms/" + id + "/name", "/"},
		{http.MethodGet, "/fragment/room/new", "GET /fragment/room/new"},
		{http.MethodPost, "/fragment/room/new", "/fragment/"},
		{http.MethodGet, "/fragment/room/members", "GET /fragment/room/members"},
		{http.MethodPost, "/rooms/" + id + "/music", "POST /rooms/{id}/music"},
		{http.MethodPost, "/rooms/" + id + "/music/play", "POST /rooms/{id}/music/play"},
		{http.MethodPost, "/rooms/" + id + "/music/pause", "POST /rooms/{id}/music/pause"},
		{http.MethodPost, "/rooms/" + id + "/music/stop", "POST /rooms/{id}/music/stop"},
		{http.MethodPost, "/rooms/" + id + "/music/loop", "POST /rooms/{id}/music/loop"},
		{http.MethodPost, "/rooms/" + id + "/music/ended", "POST /rooms/{id}/music/ended"},
		{http.MethodGet, "/rooms/" + id + "/music/audio", "GET /rooms/{id}/music/audio"},
		{http.MethodGet, "/rooms/" + id + "/music", "/"},
		{http.MethodPost, "/rooms/" + id + "/music/audio", "/"},
		{http.MethodGet, "/fragment/room/music", "GET /fragment/room/music"},
		{http.MethodGet, "/fragment/room/music/library", "GET /fragment/room/music/library"},
		{http.MethodPost, "/fragment/room/music", "/fragment/"},
		{http.MethodPost, "/fragment/room/members", "/fragment/"},
		{http.MethodGet, "/fragment/room/debug/renderer", "GET /fragment/room/debug/renderer"},
		{http.MethodGet, "/fragment/room/debug/events", "GET /fragment/room/debug/events"},
		{http.MethodGet, "/fragment/room/debug/state", "GET /fragment/room/debug/state"},
		{http.MethodGet, "/fragment/room/debug/server", "GET /fragment/room/debug/server"},
		{http.MethodPost, "/fragment/room/debug/renderer", "/fragment/"},
		{http.MethodGet, "/fragment/room/debug", "/fragment/"},
		{http.MethodGet, "/fragment/room/layers", "GET /fragment/room/layers"},
		{http.MethodGet, "/fragment/room/maps", "GET /fragment/room/maps"},
		{http.MethodGet, "/fragment/room/grid", "GET /fragment/room/grid"},
		{http.MethodGet, "/fragment/room/layer", "GET /fragment/room/layer"},
		{http.MethodPost, "/fragment/room/layers", "/fragment/"},
		{http.MethodPost, "/fragment/room/grid", "/fragment/"},
		{http.MethodDelete, "/fragment/room/layers", "/fragment/"},
		{http.MethodGet, "/rooms/" + id + "/members", "/"},
		{http.MethodGet, "/fragment/rooms/" + id, "/fragment/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

func TestCrossSiteMutationsAreRefused(t *testing.T) {
	h := handler(&controllers.App{}, middleware.Auth{})
	mutations := []struct{ method, path string }{
		{http.MethodPost, "/characters"},
		{http.MethodPost, "/rooms"},
		{http.MethodPost, "/rooms/join"},
		{http.MethodDelete, "/rooms/01BX5ZZKBKACTAV9WEVGEMMVT0"},
	}
	for _, site := range []string{"cross-site", "same-site"} {
		for _, mutation := range mutations {
			req := httptest.NewRequest(mutation.method, mutation.path, nil)
			req.Header.Set("Sec-Fetch-Site", site)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("a %s %s %s answered %d, want %d", site, mutation.method, mutation.path, rec.Code, http.StatusForbidden)
			}
		}
	}
}

func TestSameOriginMutationsReachTheSessionCheck(t *testing.T) {
	h := handler(&controllers.App{}, middleware.Auth{})
	req := httptest.NewRequest(http.MethodPost, "/characters", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("a same-origin POST answered %d, want %d -- it did not reach the session check", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/sign-in" {
		t.Errorf("Location = %q, want %q", got, "/sign-in")
	}
}

func TestTheRoomSocketRefusesWithA404RatherThanARedirect(t *testing.T) {
	h := handler(&controllers.App{}, middleware.Auth{})
	req := httptest.NewRequest(http.MethodGet, "/socket/room/01BX5ZZKBKACTAV9WEVGEMMVT0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want no redirect at all", got)
	}
}

func TestCrossSiteReadsAreNotRefused(t *testing.T) {
	h := handler(&controllers.App{}, middleware.Auth{})
	req := httptest.NewRequest(http.MethodGet, "/tos", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Error("a cross-site GET was refused; the app relies on GET being reachable from anywhere")
	}
}

func TestEveryResponseCarriesTheSecurityFloor(t *testing.T) {
	h := handler(&controllers.App{}, middleware.Auth{})
	req := httptest.NewRequest(http.MethodPost, "/characters", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestLogoutIsPostOnly(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/logout", nil)); pattern != "POST /logout" {
		t.Errorf("POST /logout matched %q, want %q", pattern, "POST /logout")
	}
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/logout", nil)); pattern == "POST /logout" {
		t.Error("GET /logout reached the logout handler")
	}
}

func TestStaticDirectoriesAreNotListed(t *testing.T) {
	h := routes(&controllers.App{}, middleware.Auth{})
	for _, path := range []string{"/css/", "/js/", "/static/", "/images/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s answered %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}
