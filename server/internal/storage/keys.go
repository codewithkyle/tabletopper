package storage

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
)


const cleanupTimeout = 15 * time.Second



























func MapPrefix(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/maps/" + assetID.String() + "/"
}



func MapGenerationPrefix(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID) string {
	return MapPrefix(userID, assetID) + gen.String() + "/"
}



func MapOriginalKey(userID ulid.ULID, assetID ulid.ULID) string {
	return MapPrefix(userID, assetID) + "original"
}


func MapPreviewKey(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID) string {
	return MapGenerationPrefix(userID, assetID, gen) + "preview"
}




func MapTileKey(userID ulid.ULID, assetID ulid.ULID, gen ulid.ULID, z int, x int, y int) string {
	return MapGenerationPrefix(userID, assetID, gen) +
		"z" + strconv.Itoa(z) + "/" +
		strconv.Itoa(x) + "_" + strconv.Itoa(y) + ".webp"
}













func AvatarKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/avatars/" + assetID.String()
}



func TokenKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/tokens/" + assetID.String()
}




func MusicKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/music/" + assetID.String()
}



func CharacterPortraitKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/portraits/" + assetID.String()
}



func JournalImageKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/journals/" + assetID.String()
}




func MonsterImageKey(userID ulid.ULID, assetID ulid.ULID) string {
	return "users/" + userID.String() + "/monsters/" + assetID.String()
}



func CleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}











func (c *Client) UploadImage(ctx context.Context, key string, body []byte) error {
	return c.Put(ctx, key, body, "image/webp")
}




func (c *Client) UploadCharacterPortrait(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.UploadImage(ctx, CharacterPortraitKey(userID, assetID), body)
}



func (c *Client) UploadJournalImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, JournalImageKey(userID, assetID), body, "image/webp")
}




func (c *Client) UploadMonsterImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body []byte) error {
	return c.Put(ctx, MonsterImageKey(userID, assetID), body, "image/webp")
}















func (c *Client) UploadMapOriginal(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, body io.ReadSeeker, size int64, contentType string) error {
	return c.PutReader(ctx, MapOriginalKey(userID, assetID), body, size, contentType)
}
