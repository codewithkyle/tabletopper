package controllers

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pngHeader builds a PNG that is a signature and an IHDR chunk and nothing
// else: no pixel data, no IDAT, no IEND. It is a valid file to
// image.DecodeConfig, which reads the dimensions out of IHDR and stops, and
// garbage to any decoder that tries to produce an image from it.
//
// That is exactly the fixture the budget needs. A real 20,000 by 20,000 PNG
// would be the thing under test committed to the repository; this is fifty
// bytes and declares the same canvas.
//
// The CRC is computed rather than hard-coded because the chunk parser checks
// it, and colour type 6 -- 8-bit RGBA -- is chosen because DecodeConfig returns
// as soon as it has IHDR for anything that is not paletted.
func pngHeader(width, height uint32) []byte {
	chunk := []byte("IHDR")
	chunk = binary.BigEndian.AppendUint32(chunk, width)
	chunk = binary.BigEndian.AppendUint32(chunk, height)
	chunk = append(chunk, 8, 6, 0, 0, 0)

	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, uint32(len(chunk)-len("IHDR")))
	out = append(out, chunk...)

	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

// uploadRequest posts content as one multipart file under field.
func uploadRequest(t *testing.T, field string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile(field, "huge.png")
	if err != nil {
		t.Fatalf("building the form: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("closing the form: %v", err)
	}

	r := httptest.NewRequest(http.MethodPost, "/assets/maps", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())

	return r
}

// THE REFUSAL HAPPENS BEFORE THE DECODE, and the pair of cases is what proves
// it rather than asserting it.
//
// Both fixtures are a header and nothing else, so neither can be decoded into
// an image. The oversized one is answered 413 from its declared dimensions --
// which the decoder never got far enough to disagree with -- and the small one
// gets past the budget and fails afterwards with the 415 a corrupt file
// deserves. A check placed after image.Decode would have answered 415 for both,
// having first allocated 1.6 GB for the one it was supposed to refuse.
func TestReadImageUploadRefusesOverThePixelBudget(t *testing.T) {
	for _, c := range []struct {
		name          string
		width, height uint32
		status        int
	}{
		{name: "over the budget", width: 20000, height: 20000, status: http.StatusRequestEntityTooLarge},
		{name: "within the budget", width: 100, height: 100, status: http.StatusUnsupportedMediaType},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := uploadRequest(t, "map", pngHeader(c.width, c.height))

			if _, _, ok := readImageUpload(rec, r, "map"); ok {
				t.Fatal("readImageUpload accepted a file with no pixels in it")
			}
			if rec.Code != c.status {
				t.Errorf("status = %d, want %d", rec.Code, c.status)
			}
			// It answered the request itself, which is what ok=false promises.
			if !strings.Contains(rec.Header().Get("HX-Trigger"), "alert") {
				t.Errorf("no alert in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
			}
		})
	}
}
