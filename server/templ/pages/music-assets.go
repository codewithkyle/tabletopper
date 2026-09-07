package pages

import "strconv"

// MusicTrack is one track as its card needs it.
//
// IT CARRIES NO DURATION AND NO WAVEFORM, and that is a decision rather than an
// omission. Reading a track's length on the server means parsing MPEG frame
// headers or Ogg granule positions -- a decoder this app does not have, for a
// file it deliberately never reads. The browser gets the number free from the
// audio element's metadata, but reporting it back would be a whole mutation
// route for something nothing needs yet.
//
// IT ALSO CARRIES NO STATE. A row whose upload never finished is not listed at
// all -- GetMusicLibrary filters on uploaded_at -- so unlike MapAsset, which has
// to describe a pyramid and a job that disagree with each other, every track
// that reaches this type is one that plays.
type MusicTrack struct {
	ID   string
	Name string
	// FileName is what the file was called when it was uploaded. It is the only
	// clue on the card about what format a track is, which matters here in a
	// way it does not for a picture: a browser that will not play one is a
	// silent player, and the extension is the first thing to look at.
	FileName string
}

// URL is the track's own resource, which its Delete posts to.
func (t MusicTrack) URL() string {
	return "/assets/music/" + t.ID
}

// NameURL is the name resource, a PATCH of its own like every other asset's.
func (t MusicTrack) NameURL() string {
	return t.URL() + "/name"
}

// AudioURL is what the <audio> element plays, and it is a route on this server
// that redirects to a freshly signed URL on the bucket rather than the signed
// URL itself.
//
// PUTTING THE SIGNED URL IN THE MARKUP WOULD NOT WORK. It expires, and a page
// left open across a session would hold a src that 403s the moment somebody
// pressed play -- or worse, mid-track, on the range request the player makes to
// refill its buffer. Going through a route means every request the player makes
// is answered with a signature minted a moment earlier. See GetMusicAudio.
func (t MusicTrack) AudioURL() string {
	return t.URL() + "/audio"
}

func (t MusicTrack) nameBox() nameBox {
	return nameBox{
		ID:        "music-name-" + t.ID,
		Value:     t.Name,
		URL:       t.NameURL(),
		Field:     "name",
		MaxLength: strconv.Itoa(AssetNameLimit),
	}
}

// MusicAccept is the file picker's filter on the music upload.
//
// THE EXTENSIONS ARE LISTED AS WELL AS THE MIME PREFIX because a browser
// matching on type alone gets it wrong often: an .m4a is reported as audio/mp4
// by some and audio/x-m4a or nothing at all by others, and a WebM audio file is
// frequently reported as video/webm. It is a hint either way -- internal/audio
// is what actually decides, twice -- but a picker that grays out the file
// somebody came to upload is worse than one that is slightly too broad.
const MusicAccept = "audio/*,.mp3,.ogg,.oga,.opus,.m4a,.mp4,.webm,.flac,.wav"
