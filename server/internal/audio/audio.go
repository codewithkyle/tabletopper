

















package audio

import (
	"path/filepath"
	"strings"
)





const HeaderBytes = 64

















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







func TypeForName(name string) (string, bool) {
	contentType, ok := byExtension[strings.ToLower(filepath.Ext(name))]

	return contentType, ok
}









func TypeForBytes(head []byte) (string, bool) {
	switch {
	
	case hasPrefix(head, "ID3"):
		return "audio/mpeg", true
	case hasPrefix(head, "OggS"):
		return "audio/ogg", true
	case hasPrefix(head, "fLaC"):
		return "audio/flac", true
	
	case len(head) >= 4 && head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3:
		return "audio/webm", true
	
	
	case len(head) >= 8 && string(head[4:8]) == "ftyp":
		return "audio/mp4", true
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "audio/wav", true
	
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		return "audio/mpeg", true
	}

	return "", false
}

func hasPrefix(head []byte, prefix string) bool {
	return len(head) >= len(prefix) && string(head[:len(prefix)]) == prefix
}
