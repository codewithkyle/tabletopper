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




func TestMapTileKeyFormat(t *testing.T) {
	got := MapTileKey(testUser, testAsset, testGen, 3, 12, 7)
	want := "users/" + testUser.String() +
		"/maps/" + testAsset.String() +
		"/" + testGen.String() + "/z3/12_7.webp"

	if got != want {
		t.Errorf("MapTileKey = %q, want %q", got, want)
	}
}





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




func TestGenerationsDoNotShareTileKeys(t *testing.T) {
	first := MapTileKey(testUser, testAsset, testGen, 2, 4, 4)
	second := MapTileKey(testUser, testAsset, testOther, 2, 4, 4)

	if first == second {
		t.Errorf("tiles of two generations share the key %q", first)
	}
}




func TestMapPrefixesEndInASlash(t *testing.T) {
	if prefix := MapPrefix(testUser, testAsset); !strings.HasSuffix(prefix, "/") {
		t.Errorf("MapPrefix = %q, want a trailing slash", prefix)
	}
	if prefix := MapGenerationPrefix(testUser, testAsset, testGen); !strings.HasSuffix(prefix, "/") {
		t.Errorf("MapGenerationPrefix = %q, want a trailing slash", prefix)
	}
}
