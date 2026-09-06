package storage

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

var (
	testUser  = ulid.MustParse("01JAAAAAAAAAAAAAAAAAAAAAA1")
	testAsset = ulid.MustParse("01JBBBBBBBBBBBBBBBBBBBBBB2")
	testGen   = ulid.MustParse("01JCCCCCCCCCCCCCCCCCCCCCC3")
	testOther = ulid.MustParse("01JDDDDDDDDDDDDDDDDDDDDDD4")
)

// The exact shape of a tile key is a wire format: it is what the tile route
// parses out of a URL and what the renderer builds one from, so a change here
// is a change to both and should not be able to happen quietly.
func TestMapTileKeyFormat(t *testing.T) {
	got := MapTileKey(testUser, testAsset, testGen, 3, 12, 7)
	want := "users/" + testUser.String() +
		"/maps/" + testAsset.String() +
		"/" + testGen.String() + "/z3/12_7.webp"

	if got != want {
		t.Errorf("MapTileKey = %q, want %q", got, want)
	}
}

// Deleting a map is DeletePrefix on its asset prefix, and that is the whole
// deletion: no key is listed anywhere else. So every key a map can own has to
// be under it, the original included -- one that escaped would be an orphan
// nothing ever looks for again.
func TestEveryMapKeyIsUnderTheAssetPrefix(t *testing.T) {
	prefix := MapPrefix(testUser, testAsset)

	for name, key := range map[string]string{
		"original": MapOriginalKey(testUser, testAsset),
		"preview":  MapPreviewKey(testUser, testAsset, testGen),
		"tile":     MapTileKey(testUser, testAsset, testGen, 0, 0, 0),
		"prefix":   MapGenerationPrefix(testUser, testAsset, testGen),
	} {
		if !strings.HasPrefix(key, prefix) {
			t.Errorf("%s key %q is not under %q", name, key, prefix)
		}
	}
}

// A superseded generation is deleted while the map lives on, so the split has
// to fall in exactly one place: everything the pyramid produced goes, and the
// original -- the source every future re-tile decodes -- stays.
func TestGenerationPrefixCoversThePyramidAndNotTheOriginal(t *testing.T) {
	prefix := MapGenerationPrefix(testUser, testAsset, testGen)

	if key := MapPreviewKey(testUser, testAsset, testGen); !strings.HasPrefix(key, prefix) {
		t.Errorf("preview key %q is not under %q", key, prefix)
	}
	if key := MapTileKey(testUser, testAsset, testGen, 5, 1, 1); !strings.HasPrefix(key, prefix) {
		t.Errorf("tile key %q is not under %q", key, prefix)
	}
	if key := MapOriginalKey(testUser, testAsset); strings.HasPrefix(key, prefix) {
		t.Errorf("original key %q is under %q; deleting a generation would take it", key, prefix)
	}
}

// Two generations of the same map never share a key, which is what lets the
// old one keep serving while the new one is built and what makes a tile URL
// safe to cache for a year.
func TestGenerationsDoNotShareTileKeys(t *testing.T) {
	first := MapTileKey(testUser, testAsset, testGen, 2, 4, 4)
	second := MapTileKey(testUser, testAsset, testOther, 2, 4, 4)

	if first == second {
		t.Errorf("tiles of two generations share the key %q", first)
	}
}

// DeletePrefix refuses anything that does not end in "/", so a prefix builder
// that dropped one would turn every deletion into an error rather than into a
// wider delete than intended.
func TestMapPrefixesEndInASlash(t *testing.T) {
	if prefix := MapPrefix(testUser, testAsset); !strings.HasSuffix(prefix, "/") {
		t.Errorf("MapPrefix = %q, want a trailing slash", prefix)
	}
	if prefix := MapGenerationPrefix(testUser, testAsset, testGen); !strings.HasSuffix(prefix, "/") {
		t.Errorf("MapGenerationPrefix = %q, want a trailing slash", prefix)
	}
}
