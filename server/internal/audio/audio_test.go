package audio

import "testing"







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
		
		
		"opus is ogg": {"tavern.opus", "audio/ogg", true},
		"m4a is mp4":  {"march.m4a", "audio/mp4", true},
		
		
		
		"webm":                      {"ripped.webm", "audio/webm", true},
		"flac":                      {"lossless.flac", "audio/flac", true},
		"wav":                       {"thunder.wav", "audio/wav", true},
		"case does not matter":      {"BATTLE.MP3", "audio/mpeg", true},
		"a dot in the name is fine": {"act 2 - the keep.mp3", "audio/mpeg", true},
		"no extension":              {"battle", "", false},
		"an image":                  {"battle.png", "", false},
		
		
		
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
		
		
		"a riff that is not wave": {append(append([]byte("RIFF"), 0x24, 0, 0, 0), []byte("AVI ")...), "", false},
		"a png":                   {[]byte("\x89PNG\r\n\x1a\n"), "", false},
		"nothing at all":          {nil, "", false},
		
		
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
