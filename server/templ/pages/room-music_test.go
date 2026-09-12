package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/uievents"
)

const testMusicRoomID = "01BX5ZZKBKACTAV9WEVGEMMVX0"

func musicWindow(gm bool) RoomMusicData {
	return RoomMusicData{
		RoomID:     testMusicRoomID,
		CanControl: gm,
		Loaded:     true,
		Name:       "Tavern Brawl",
		Playing:    true,
		Tracks:     []MusicTrack{{ID: "01TRACK", Name: "Tavern Brawl", FileName: "tavern.mp3"}},
	}
}

func TestTheNowPlayingBlockRefetchesItselfAndNothingElse(t *testing.T) {
	markup := decoded(t, RoomMusic(musicWindow(true)))
	if !strings.Contains(markup, `hx-trigger="`+uievents.Music+` from:window"`) {
		t.Errorf("the block does not listen for %q:\n%s", uievents.Music, markup)
	}
	if !strings.Contains(markup, `hx-select="#`+RoomMusicNowID+`"`) {
		t.Error("the block does not select itself out of the response, so a refetch would reset the volume and the search")
	}
	if !strings.Contains(markup, `hx-sync="this:queue last"`) {
		t.Error("the block does not queue its refetches")
	}
}
func TestEveryTransportButtonLeavesTheRepaintToTheSocket(t *testing.T) {
	markup := decoded(t, RoomMusic(musicWindow(true)))
	posts := strings.Count(markup, "hx-post=")
	if swaps := strings.Count(markup, `hx-swap="none"`); swaps != posts {
		t.Errorf("%d of %d posts swap their own reply; the socket is what repaints every window:\n%s", posts-swaps, posts, markup)
	}
}
func TestThePlayersWindowHearsTheMusicWithoutRunningIt(t *testing.T) {
	markup := decoded(t, RoomMusic(musicWindow(false)))
	for _, want := range []string{"Tavern Brawl", "data-music-bar", "data-music-volume", "data-music-elapsed"} {
		if !strings.Contains(markup, want) {
			t.Errorf("a player cannot see %q:\n%s", want, markup)
		}
	}
	for _, deny := range []string{"hx-post", "data-music-file", "data-music-search", "Repeat"} {
		if strings.Contains(markup, deny) {
			t.Errorf("a player was sent %q, which is the GM's alone:\n%s", deny, markup)
		}
	}
}
func TestTheGMsWindowCarriesTheTransportAndTheLibrary(t *testing.T) {
	markup := decoded(t, RoomMusic(musicWindow(true)))
	for _, want := range []string{
		"/rooms/" + testMusicRoomID + "/music/pause",
		"/rooms/" + testMusicRoomID + "/music/stop",
		"/rooms/" + testMusicRoomID + "/music/loop",
		"/fragment/room/music/library?room=" + testMusicRoomID,
		"data-music-file",
		"Repeat",
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the GM's window is missing %q:\n%s", want, markup)
		}
	}
}
func TestTheButtonMatchesWhatTheTrackIsDoing(t *testing.T) {
	playing := decoded(t, RoomMusic(musicWindow(true)))
	if !strings.Contains(playing, "/music/pause") || strings.Contains(playing, `"/rooms/`+testMusicRoomID+`/music/play"`) {
		t.Errorf("a playing track is not offering a pause:\n%s", playing)
	}
	data := musicWindow(true)
	data.Playing = false
	data.Started = true
	paused := decoded(t, RoomMusic(data))
	if !strings.Contains(paused, "/music/play") || strings.Contains(paused, "/music/pause") {
		t.Errorf("a paused track is not offering a play:\n%s", paused)
	}
	if !strings.Contains(paused, ">Paused<") {
		t.Errorf("a paused track does not say so:\n%s", paused)
	}
}
func TestNothingLoadedLeavesTheTransportDead(t *testing.T) {
	data := musicWindow(true)
	data.Loaded = false
	data.Playing = false
	data.Name = ""
	markup := decoded(t, RoomMusic(data))
	if !strings.Contains(markup, "Nothing loaded") {
		t.Errorf("an empty window does not say it is empty:\n%s", markup)
	}
	if strings.Count(markup, "disabled") < 2 {
		t.Errorf("play and stop are live with nothing to play:\n%s", markup)
	}
}
func TestATrackInTheListLoadsItselfByID(t *testing.T) {
	markup := decoded(t, RoomMusicList(musicWindow(true)))
	if !strings.Contains(markup, `hx-post="/rooms/`+testMusicRoomID+`/music"`) {
		t.Errorf("the row does not post to the room's music:\n%s", markup)
	}
	if !strings.Contains(markup, `hx-vals="{"track":"01TRACK"}"`) {
		t.Errorf("the row does not name the track it loads:\n%s", markup)
	}
}
func TestAnEmptyLibraryReadsDifferentlyFromAnEmptySearch(t *testing.T) {
	data := musicWindow(true)
	data.Tracks = nil
	if markup := decoded(t, RoomMusicList(data)); !strings.Contains(markup, "No music in your library yet") {
		t.Errorf("an empty library does not say how to fill it:\n%s", markup)
	}
	data.Query = "tavern"
	if markup := decoded(t, RoomMusicList(data)); !strings.Contains(markup, "No track of yours is called that") {
		t.Errorf("a search with no matches reads as an empty library:\n%s", markup)
	}
}
