package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
)

// cleanupTimeout bounds a compensating delete after a failed upload.
const cleanupTimeout = 15 * time.Second

// MapKeys returns the keys holding a map's full-size image and its preview, in
// the flat layout the upload path still writes.
func MapKeys(userID ulid.ULID, assetID ulid.ULID) (full string, preview string) {
	base := "users/" + userID.String() + "/maps/"
	return base + assetID.String(), base + "preview-" + assetID.String()
}

// A tiled map is a directory rather than a pair of keys, because it is a
// pyramid rather than one image:
//
//	users/{userID}/maps/{assetID}/original
//	users/{userID}/maps/{assetID}/{gen}/preview
//	users/{userID}/maps/{assetID}/{gen}/z{z}/{x}_{y}.webp
//
// gen names one generation: a complete pyramid, written once and never
// modified. Replacing a map, retrying a failed build and re-tiling at a
// different tile size each mint a new one, so the tiles a browser has cached
// under the old generation's URLs can never be contradicted by the new ones.
// The old generation keeps serving until the new one is finished, and is
// deleted then.
//
// The preview lives under its generation because it is built from it, so it
// goes when that pyramid goes. The original does not: it is the source every
// re-tile decodes, it is stored exactly as it was uploaded rather than
// re-encoded, and it is never served -- the image routes answer image/webp and
// the original is whatever PNG or JPEG arrived.
//
// The two prefixes are what deletion needs. A generation's, when a newer one
// supersedes it, and the asset's, when the map goes: one list and one batch
// delete each, rather than a call per tile.

// MapPrefix returns the prefix holding everything a map owns -- its original
// and every generation of its pyramid.
func MapPrefix(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/maps/" + assetID.String() + "/"
}

// MapGenerationPrefix returns the prefix holding one generation: its preview
// and every tile of it.
func MapGenerationPrefix(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID) string {
	return MapPrefix(userID, assetID) + gen.String() + "/"
}

// MapOriginalKey returns the key holding the file as it was uploaded. It
// belongs to the map rather than to any generation, and outlives all of them.
func MapOriginalKey(userID ulid.ULID, assetID ulid.ULID) string {
	return MapPrefix(userID, assetID) + "original"
}

// MapPreviewKey returns the key holding one generation's preview thumbnail.
func MapPreviewKey(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID) string {
	return MapGenerationPrefix(userID, assetID, gen) + "preview"
}

// MapTileKey returns the key holding one tile of one generation. z is the
// pyramid level, where 0 is native resolution and each level up is halved, and
// x and y index the tile grid of that level.
func MapTileKey(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID, z int, x int, y int) string {
	return MapGenerationPrefix(userID, assetID, gen) +
		"z" + strconv.Itoa(z) + "/" +
		strconv.Itoa(x) + "_" + strconv.Itoa(y) + ".webp"
}

// AvatarKey returns the key holding a character's avatar.
func AvatarKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/avatars/" + assetID.String()
}

// JournalImageKey returns the key holding one journal image. There is no
// preview: the image is served at the size it was stored.
func JournalImageKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/journals/" + assetID.String()
}

// CleanupContext detaches from the request so a compensating delete still runs
// when the upload failed because the client disconnected.
func CleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}

// UploadAvatar writes a character avatar to the key returned by AvatarKey. The
// asset row must already exist so a failure here can be cleaned up.
func (c *Client) UploadAvatar(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, AvatarKey(userID, assetID), body, "image/webp")
}

// UploadJournalImage writes one journal image. The asset row must already
// exist so a failure here can be cleaned up.
func (c *Client) UploadJournalImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, JournalImageKey(userID, assetID), body, "image/webp")
}

// UploadMap writes a map's full-size image and preview to the keys returned by
// MapKeys. The asset row must already exist so a failure here can be cleaned up.
func (c *Client) UploadMap(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, full []byte, preview []byte) error {
	fullKey, previewKey := MapKeys(userID, assetID)
	if err := c.Put(ctx, fullKey, full, "image/webp"); err != nil {
		return err
	}
	return c.Put(ctx, previewKey, preview, "image/webp")
}

// DeleteMapObjects removes both of a map's objects. Deleting a key that was
// never written succeeds, so this is safe after a partially completed upload.
func (c *Client) DeleteMapObjects(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) error {
	fullKey, previewKey := MapKeys(userID, assetID)
	if err := c.Delete(ctx, fullKey); err != nil {
		return err
	}
	return c.Delete(ctx, previewKey)
}
