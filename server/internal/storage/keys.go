package storage

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
)

// cleanupTimeout bounds a compensating delete after a failed upload.
const cleanupTimeout = 15 * time.Second

// A map is a directory rather than a pair of keys, because it is a tile
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

// AvatarKey returns the key holding one avatar in the account's library: a face
// the game master gathered to put on an NPC, owned by nobody until one is
// spawned. Flat like a monster's picture and not a directory like a map.
//
// THIS PREFIX ALSO HOLDS EVERY CHARACTER PORTRAIT WRITTEN BEFORE THE LIBRARY
// EXISTED, and that is deliberate. A portrait used to be an `avatar` and used
// to be stored here; 20260907120000 moved the rows to their own member and
// moved nothing in the bucket, because every read builds its key from
// assets.file_path rather than from the type. New portraits go to
// CharacterPortraitKey. The two cannot collide -- an asset id is a ULID and
// names exactly one row -- so the only cost is a prefix holding two kinds of
// thing, which is cheaper than copying objects to make a listing tidy.
func AvatarKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/avatars/" + assetID.String()
}

// TokenKey returns the key holding one token: a thing on the board that is not
// a creature, kept at whatever shape it was uploaded in.
func TokenKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/tokens/" + assetID.String()
}

// MusicKey returns the key holding one track. Flat like a token, and the only
// object in the bucket this process never writes: the browser PUTs it through a
// presigned URL, so what is here is the key that URL is signed for.
func MusicKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/music/" + assetID.String()
}

// CharacterPortraitKey returns the key holding a character's portrait. See
// AvatarKey for why portraits written before the library existed are not here.
func CharacterPortraitKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/portraits/" + assetID.String()
}

// JournalImageKey returns the key holding one journal image. There is no
// preview: the image is served at the size it was stored.
func JournalImageKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/journals/" + assetID.String()
}

// MonsterImageKey returns the key holding a monster's picture. Flat like an
// avatar and not a directory like a map: it is one image, stored at the size it
// is served at, with no pyramid and no generation to keep apart.
func MonsterImageKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/monsters/" + assetID.String()
}

// CleanupContext detaches from the request so a compensating delete still runs
// when the upload failed because the client disconnected.
func CleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}

// UploadImage writes one stored image to a key its caller has already built.
//
// IT TAKES A KEY RATHER THAN AN ID because two callers cannot build one. A
// library upload's key comes from the kind it was routed as, and a replacement
// overwrites the key on the row it is replacing -- which for a portrait written
// before the library existed is not the key any builder here would produce.
//
// image/webp is pinned rather than passed, because it is true of every image
// this app stores: everything but a map's original goes through
// images.EncodeWebP on the way in.
func (c *Client) UploadImage(ctx context.Context, key string, body []byte) error {
	return c.Put(ctx, key, body, "image/webp")
}

// UploadCharacterPortrait writes a character's portrait to the key returned by
// CharacterPortraitKey. The asset row must already exist so a failure here can
// be cleaned up.
func (c *Client) UploadCharacterPortrait(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.UploadImage(ctx, CharacterPortraitKey(userID, assetID), body)
}

// UploadJournalImage writes one journal image. The asset row must already
// exist so a failure here can be cleaned up.
func (c *Client) UploadJournalImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, JournalImageKey(userID, assetID), body, "image/webp")
}

// UploadMonsterImage writes a monster's picture to the key returned by
// MonsterImageKey. The asset row must already exist so a failure here can be
// cleaned up.
func (c *Client) UploadMonsterImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, MonsterImageKey(userID, assetID), body, "image/webp")
}

// UploadMapOriginal writes a map's file as it was uploaded, at the key that
// belongs to the map rather than to any of its generations. The asset row must
// already exist so a failure here can be cleaned up.
//
// IT TAKES A READER AND NOT BYTES, alone among the uploads here, because this
// is the only one whose body was not produced by this process -- see PutReader.
// The caller hands over the multipart.File it was given and its declared size,
// and 128 MiB goes from the temp file to the bucket without passing through the
// heap.
//
// contentType is what the upload's header said it is, not image/webp: this is
// the one stored image that is never re-encoded, and it is a PNG or a JPEG as
// often as not. Nothing serves it -- the image routes answer image/webp -- so
// the type is here for whoever is looking in the bucket.
func (c *Client) UploadMapOriginal(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body io.ReadSeeker, size int64, contentType string) error {
	return c.PutReader(ctx, MapOriginalKey(userID, assetID), body, size, contentType)
}
