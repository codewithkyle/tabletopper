// Package audio is the allowlist for uploaded music: what an extension claims a
// file is, and what its first bytes say it actually is.
//
// IT EXISTS BECAUSE THERE IS NO DECODER REGISTRY FOR AUDIO. An uploaded image
// is checked by image.DecodeConfig, which is backed by the decoders the process
// registered, so "a format we can read" and "a format we accept" are the same
// list and cannot drift apart. Nothing in the standard library does that for
// sound, so the allowlist has to be written down -- and written down twice, once
// as the extensions a name may end in and once as the bytes a file may start
// with, because the two checks happen at different moments.
//
// THE TWO CHECKS BRACKET A PRESIGNED UPLOAD. TypeForName runs before the URL is
// signed, because the signature pins a Content-Type and R2 stores whatever it is
// given -- so the type has to be decided from the only thing known at that
// point, which is the filename. TypeForBytes runs after the object lands, on the
// first 64 bytes read back out of the bucket, and is what stops the name being
// taken at its word. A file called track.mp3 that is really a WebM would
// otherwise be stored as audio/mpeg and served as audio/mpeg forever.
package audio

import (
	"path/filepath"
	"strings"
)

// HeaderBytes is how much of an object has to be read back to identify it. The
// longest signature here is twelve bytes -- RIFF....WAVE -- and 64 is a round
// number well clear of it, small enough that reading it is one ranged GET of
// nothing.
const HeaderBytes = 64

// byExtension is what a name is allowed to end in, and the Content-Type each one
// is stored and served as.
//
// .opus AND .oga ARE audio/ogg, not audio/opus. Opus is a codec inside an Ogg
// container, the container is what the bytes say, and audio/ogg is what every
// browser dispatches on. The same reasoning puts .m4a under audio/mp4.
//
// .webm IS NOT OPTIONAL HERE. yt-dlp's default audio output is frequently Opus
// in a WebM container, which is not an Ogg page and would be refused by a list
// that had only thought about .mp3 and .ogg.
//
// .aac IS DELIBERATELY ABSENT. A bare .aac is usually raw ADTS, whose first two
// bytes are 0xFF 0xF1 -- indistinguishable from an MPEG frame sync, so the
// second check could not tell it from an MP3 and would have to either accept
// both under one type or refuse a valid file. .m4a is what these files actually
// arrive as.
var byExtension = map[string]string{
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".m4a":  "audio/mp4",
	".mp4":  "audio/mp4",
	".webm": "audio/webm",
	".flac": "audio/flac",
	".wav":  "audio/wav",
}

// TypeForName reports the Content-Type an uploaded filename claims, and whether
// it claims one this app accepts at all.
//
// It is the check made BEFORE anything is signed, so a name this refuses costs
// the uploader nothing: no row is written, no URL is minted, and the file never
// leaves their machine.
func TypeForName(name string) (string, bool) {
	contentType, ok := byExtension[strings.ToLower(filepath.Ext(name))]

	return contentType, ok
}

// TypeForBytes reports the Content-Type the start of a file actually is.
//
// head is the first bytes of the object, and may be short; every case checks its
// own length, so a truncated or empty read is refused rather than read past.
//
// THE MPEG FRAME SYNC IS LAST BECAUSE IT IS THE LOOSEST TEST. It is eleven set
// bits and nothing else, which is a pattern that can occur by accident, so every
// signature with real structure to it gets to answer first.
func TypeForBytes(head []byte) (string, bool) {
	switch {
	// An ID3 tag, which is how most MP3s in the wild begin.
	case hasPrefix(head, "ID3"):
		return "audio/mpeg", true
	case hasPrefix(head, "OggS"):
		return "audio/ogg", true
	case hasPrefix(head, "fLaC"):
		return "audio/flac", true
	// EBML, the container WebM and Matroska share.
	case len(head) >= 4 && head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3:
		return "audio/webm", true
	// An ISO base media file -- .m4a, .mp4 -- whose first box is ftyp. The
	// four bytes before it are that box's length.
	case len(head) >= 8 && string(head[4:8]) == "ftyp":
		return "audio/mp4", true
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "audio/wav", true
	// A bare MPEG audio frame: eleven bits of sync and nothing else to check.
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		return "audio/mpeg", true
	}

	return "", false
}

func hasPrefix(head []byte, prefix string) bool {
	return len(head) >= len(prefix) && string(head[:len(prefix)]) == prefix
}
