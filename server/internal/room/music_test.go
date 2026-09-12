package room

import "testing"

func playing(w *world) change {
	w.t.Helper()
	return w.change(&MusicLoad{AssetID: testTrackID, Track: &TrackInfo{Name: "Tavern Brawl"}}, w.gm)
}

func TestLoadingATrackStartsItForTheWholeTable(t *testing.T) {
	w := newWorld(t)
	ch := playing(w)
	music := w.s.Music
	if music.TrackID == nil || *music.TrackID != testTrackID {
		t.Fatalf("the room is holding %+v, want the track it was given", music.TrackID)
	}
	if music.Name != "Tavern Brawl" {
		t.Errorf("name = %q, want the library's own", music.Name)
	}
	if !music.Playing || music.At != 0 || music.Since == 0 {
		t.Errorf("the track is %+v, want it playing from the start at a known moment", music)
	}
	for _, role := range []Role{RoleGM, RolePlayer} {
		equalStrings(t, string(role)+" changes", changeTypesOf(ch.changes(role)), []string{"music.updated"})
	}
}
func TestPausingKeepsThePlaceAndPlayingPicksItUp(t *testing.T) {
	w := newWorld(t)
	playing(w)
	started := w.s.Music.Since
	w.apply(&MusicPause{}, w.gm)
	paused := w.s.Music
	if paused.Playing || paused.Since != 0 {
		t.Fatalf("the track is %+v, want it stopped where it was", paused)
	}
	if paused.At != testClockStep {
		t.Errorf("at = %d, want the %d milliseconds it ran for", paused.At, testClockStep)
	}
	w.apply(&MusicPlay{}, w.gm)
	again := w.s.Music
	if !again.Playing || again.At != paused.At {
		t.Errorf("the track is %+v, want it playing on from %d", again, paused.At)
	}
	if again.Since <= started {
		t.Errorf("since = %d, want a moment after the %d it first started", again.Since, started)
	}
}
func TestStoppingRewindsTheTrackWithoutUnloadingIt(t *testing.T) {
	w := newWorld(t)
	playing(w)
	w.apply(&MusicStop{}, w.gm)
	music := w.s.Music
	if music.TrackID == nil {
		t.Fatal("stopping unloaded the track; the GM should still see what is queued up")
	}
	if music.Playing || music.At != 0 || music.Since != 0 {
		t.Errorf("the track is %+v, want it back at the beginning and silent", music)
	}
}
func TestATrackOnlyEndsWhenItIsTheOneStillPlaying(t *testing.T) {
	w := newWorld(t)
	playing(w)
	w.apply(&MusicEnded{TrackID: testAssetID}, w.gm)
	if !w.s.Music.Playing {
		t.Error("a report about another track stopped the one that is playing")
	}
	w.apply(&MusicSetLoop{Loop: true}, w.gm)
	w.apply(&MusicEnded{TrackID: testTrackID}, w.gm)
	if !w.s.Music.Playing {
		t.Error("a repeating track was stopped by the end it is meant to run past")
	}
	w.apply(&MusicSetLoop{Loop: false}, w.gm)
	w.apply(&MusicEnded{TrackID: testTrackID}, w.gm)
	music := w.s.Music
	if music.Playing || music.At != 0 {
		t.Errorf("the track is %+v, want it finished and rewound", music)
	}
}
func TestRepeatOutlivesTheTrackItWasSetOn(t *testing.T) {
	w := newWorld(t)
	w.apply(&MusicSetLoop{Loop: true}, w.gm)
	playing(w)
	if !w.s.Music.Loop {
		t.Error("loading the next track forgot that the table wants it repeated")
	}
}
func TestStartingWithNothingLoadedIsRefused(t *testing.T) {
	w := newWorld(t)
	w.refuse(&MusicPlay{}, w.gm, CodeNotFound)
}
func TestATrackTheLibraryCouldNotAnswerForIsRefused(t *testing.T) {
	w := newWorld(t)
	w.refuse(&MusicLoad{AssetID: testTrackID}, w.gm, CodeNotFound)
	if w.s.Music.TrackID != nil {
		t.Error("a refused load still put something on")
	}
}
func TestAPausedTrackCountsNothingAndAPlayingOneCountsFromWhenItStarted(t *testing.T) {
	paused := Music{At: 4_000}
	if got := paused.Elapsed(1_000_000); got != 4_000 {
		t.Errorf("elapsed = %d, want the %d it was paused at", got, paused.At)
	}
	live := Music{At: 4_000, Playing: true, Since: 1_000_000}
	if got := live.Elapsed(1_006_500); got != 10_500 {
		t.Errorf("elapsed = %d, want 10500", got)
	}
	if got := live.Elapsed(999_000); got != 4_000 {
		t.Errorf("elapsed = %d, want the place it started from; a clock that ran backwards is not a rewind", got)
	}
}
