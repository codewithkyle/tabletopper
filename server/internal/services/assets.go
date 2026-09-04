package services

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// cleanupTimeout bounds a compensating delete after a failed upload.
const cleanupTimeout = 15 * time.Second

// MapKeys returns the R2 keys holding a map's full-size image and its preview.
func MapKeys(userId ulid.ULID, assetId ulid.ULID) (string, string) {
	base := "users/" + userId.String() + "/maps/"
	return base + assetId.String(), base + "preview-" + assetId.String()
}

// AvatarKey returns the R2 key holding a character's avatar.
func AvatarKey(userId ulid.ULID, assetId ulid.ULID) string {
	return "users/" + userId.String() + "/avatars/" + assetId.String()
}

// CleanupContext detaches from the request so a compensating delete still runs
// when the upload failed because the client disconnected.
func CleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}

func DeleteImage(ctx context.Context, key string) error {
	client, err := NewR2Client(ctx)
	if err != nil {
		return err
	}
	err = client.DeleteObject(ctx, key)
	return err
}

func GetImage(ctx context.Context, key string) ([]byte, error) {
	client, err := NewR2Client(ctx)
	if err != nil {
		return []byte{}, err
	}

	data, err := client.ReadBytes(ctx, key)
	if err != nil {
		return []byte{}, err
	}

	return data, err
}

// UploadAvatar writes a character avatar to the key returned by AvatarKey. The
// asset row must already exist so a failure here can be cleaned up.
func UploadAvatar(ctx context.Context, userId ulid.ULID, assetId ulid.ULID, body []byte) error {
	client, err := NewR2Client(ctx)
	if err != nil {
		return err
	}
	return client.UploadBytes(ctx, AvatarKey(userId, assetId), body, "image/webp")
}

// UploadMap writes a map's full-size image and preview to the keys returned by
// MapKeys. The asset row must already exist so a failure here can be cleaned up.
func UploadMap(ctx context.Context, userId ulid.ULID, assetId ulid.ULID, full []byte, preview []byte) error {
	client, err := NewR2Client(ctx)
	if err != nil {
		return err
	}
	fullKey, previewKey := MapKeys(userId, assetId)
	if err := client.UploadBytes(ctx, fullKey, full, "image/webp"); err != nil {
		return err
	}
	return client.UploadBytes(ctx, previewKey, preview, "image/webp")
}

// DeleteMapObjects removes both of a map's objects. Deleting a key that was
// never written succeeds, so this is safe after a partially completed upload.
func DeleteMapObjects(ctx context.Context, userId ulid.ULID, assetId ulid.ULID) error {
	client, err := NewR2Client(ctx)
	if err != nil {
		return err
	}
	fullKey, previewKey := MapKeys(userId, assetId)
	if err := client.DeleteObject(ctx, fullKey); err != nil {
		return err
	}
	return client.DeleteObject(ctx, previewKey)
}
