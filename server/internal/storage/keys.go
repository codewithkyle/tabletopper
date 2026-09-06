package storage

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// cleanupTimeout bounds a compensating delete after a failed upload.
const cleanupTimeout = 15 * time.Second

// MapKeys returns the keys holding a map's full-size image and its preview.
func MapKeys(userID ulid.ULID, assetID ulid.ULID) (full string, preview string) {
	base := "users/" + userID.String() + "/maps/"
	return base + assetID.String(), base + "preview-" + assetID.String()
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
