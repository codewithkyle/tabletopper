package audio

import "testing"

// THE ALLOWLIST IS WRITTEN DOWN TWICE AND THE TWO HALVES HAVE TO AGREE. There
// is no decoder registry for audio, so unlike an image -- where "a format we can
// read" and "a format we accept" are one list by construction -- these are two
// independent tables that can drift. Every type a name may claim has to be a
// type some byte signature can produce, or a file of that kind uploads and is
// then refused by the confirm as not being what its name says.
func TestEveryAcceptedNameHasASignatureThatCanMatchIt(t *testing.T) {
	producible := map[string]bool{}
	for _, head := range [][]byte{
		[]byte("ID3\x03"),
		{0xFF, 0xFB},
		[]byte("OggS"),
		[]byte("fLaC"),
		{0x1A, 0x45, 0xDF, 0xA3},
		append([]byte{0, 0, 0, 0x20}, []byte("ftypM4A ")...),
		append(append([]byte("RIFF"), 0, 0, 0, 0), []byte("WAVE")...),
	} {
		got, ok := TypeForBytes(head)
		if !ok {
			t.Fatalf("a signature this package writes is one it cannot read: %v", head)
		}
		producible[got] = true
	}

	for ext, contentType := range byExtension {
		if !producible[contentType] {
			t.Errorf("%s is accepted as %s, which no signature produces -- every upload of one would be refused by the confirm", ext, contentType)
		}
	}
}

func TestTypeForName(t *testing.T) {
	for name, c := range map[string]struct {
		file string
		want string
		ok   bool
	}{
		"mp3": {"battle.mp3", "audio/mpeg", true},
		"ogg": {"rain.ogg", "audio/ogg", true},
		// Opus is a codec in an Ogg container, and audio/ogg is what browsers
		// dispatch on.
		"opus is ogg": {"tavern.opus", "audio/ogg", true},
		"m4a is mp4":  {"march.m4a", "audio/mp4", true},
		// yt-dlp's default audio output is frequently Opus in WebM, which is
		// not an Ogg page. A list that had only considered mp3 and ogg would
		// refuse most of what these files actually are.
		"webm":                      {"ripped.webm", "audio/webm", true},
		"flac":                      {"lossless.flac", "audio/flac", true},
		"wav":                       {"thunder.wav", "audio/wav", true},
		"case does not matter":      {"BATTLE.MP3", "audio/mpeg", true},
		"a dot in the name is fine": {"act 2 - the keep.mp3", "audio/mpeg", true},
		"no extension":              {"battle", "", false},
		"an image":                  {"battle.png", "", false},
		// Raw ADTS starts 0xFF 0xF1, which is indistinguishable from an MPEG
		// frame sync -- so it is refused by name rather than accepted and then
		// mislabelled by the confirm.
		"bare aac": {"battle.aac", "", false},
		"a video":  {"battle.mkv", "", false},
		"empty":    {"", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := TypeForName(c.file)
			if ok != c.ok || got != c.want {
				t.Errorf("TypeForName(%q) = %q, %v; want %q, %v", c.file, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestTypeForBytes(t *testing.T) {
	for name, c := range map[string]struct {
		head []byte
		want string
		ok   bool
	}{
		"an ID3 tag":   {[]byte("ID3\x03\x00\x00\x00"), "audio/mpeg", true},
		"a frame sync": {[]byte{0xFF, 0xFB, 0x90, 0x00}, "audio/mpeg", true},
		"an ogg page":  {[]byte("OggS\x00\x02\x00\x00"), "audio/ogg", true},
		"flac":         {[]byte("fLaC\x00\x00\x00\x22"), "audio/flac", true},
		"ebml":         {[]byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x00}, "audio/webm", true},
		"an ftyp box":  {append([]byte{0, 0, 0, 0x20}, []byte("ftypM4A ")...), "audio/mp4", true},
		"a riff wave":  {append(append([]byte("RIFF"), 0x24, 0, 0, 0), []byte("WAVEfmt ")...), "audio/wav", true},
		// A RIFF that is not a WAVE -- an AVI, say -- is not audio, and the
		// second four bytes are the only thing that says so.
		"a riff that is not wave": {append(append([]byte("RIFF"), 0x24, 0, 0, 0), []byte("AVI ")...), "", false},
		"a png":                   {[]byte("\x89PNG\r\n\x1a\n"), "", false},
		"nothing at all":          {nil, "", false},
		// Every case checks its own length, so a truncated read is refused
		// rather than read past the end of the slice.
		"a truncated ogg":  {[]byte("Og"), "", false},
		"a truncated ftyp": {[]byte{0, 0, 0, 0x20, 'f', 't'}, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := TypeForBytes(c.head)
			if ok != c.ok || got != c.want {
				t.Errorf("TypeForBytes(%v) = %q, %v; want %q, %v", c.head, got, ok, c.want, c.ok)
			}
		})
	}
}

// A NAME THAT LIES IS THE WHOLE REASON THERE ARE TWO CHECKS. The Content-Type is
// signed from the filename, because that is all that is known before the file
// exists, and R2 stores and serves whatever it was given -- so a WebM called
// track.mp3 would be served as audio/mpeg for good. The confirm compares these
// two, and this is the disagreement it is looking for.
func TestANameCanDisagreeWithTheBytes(t *testing.T) {
	declared, ok := TypeForName("battle.mp3")
	if !ok {
		t.Fatal("battle.mp3 is not accepted by name")
	}
	actual, ok := TypeForBytes([]byte{0x1A, 0x45, 0xDF, 0xA3})
	if !ok {
		t.Fatal("an EBML header is not recognised")
	}
	if declared == actual {
		t.Error("a WebM named .mp3 is indistinguishable from an MP3, so the confirm cannot catch it")
	}
}
