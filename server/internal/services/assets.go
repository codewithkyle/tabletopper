package services

import (
	"context"

	"github.com/oklog/ulid/v2"
)

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

func UploadAvatar(ctx context.Context, userId ulid.ULID, assetId ulid.ULID, body []byte) (string, error) {
	client, err := NewR2Client(ctx)
	if err != nil {
		return "", err
	}
	key := "users/"+userId.String()+"/avatars/"+assetId.String()
	err = client.UploadBytes(
		ctx,
		key,
		body,
		"image/webp",
	)
	if err != nil {
		return "", err
	}
	return key, nil
}
