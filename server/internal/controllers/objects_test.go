package controllers

import (
	"bytes"
	"context"
	"io"
	"time"

	"tabletopper/internal/storage"

	"github.com/oklog/ulid/v2"
)

type memObjects map[string][]byte

func (m memObjects) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	body, ok := m[key]
	if !ok {
		return nil, -1, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(body)), int64(len(body)), nil
}
func (m memObjects) Peek(_ context.Context, key string, n int64) ([]byte, error) {
	body, ok := m[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return body[:min(n, int64(len(body)))], nil
}
func (m memObjects) Size(_ context.Context, key string) (int64, error) {
	body, ok := m[key]
	if !ok {
		return 0, storage.ErrNotFound
	}
	return int64(len(body)), nil
}
func (m memObjects) PresignPut(_ context.Context, key string, _ string, _ int64, _ time.Duration) (string, error) {
	return "https://objects.test/" + key, nil
}
func (m memObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://objects.test/" + key, nil
}
func (m memObjects) UploadImage(_ context.Context, key string, body []byte) error {
	m[key] = body
	return nil
}
func (m memObjects) UploadCharacterPortrait(ctx context.Context, userID, assetID ulid.ULID, body []byte) error {
	return m.UploadImage(ctx, storage.CharacterPortraitKey(userID, assetID), body)
}
func (m memObjects) UploadJournalImage(ctx context.Context, userID, assetID ulid.ULID, body []byte) error {
	return m.UploadImage(ctx, storage.JournalImageKey(userID, assetID), body)
}
func (m memObjects) UploadMonsterImage(ctx context.Context, userID, assetID ulid.ULID, body []byte) error {
	return m.UploadImage(ctx, storage.MonsterImageKey(userID, assetID), body)
}
func (m memObjects) UploadMapOriginal(_ context.Context, userID, assetID ulid.ULID, body io.ReadSeeker, _ int64, _ string) error {
	read, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m[storage.MapOriginalKey(userID, assetID)] = read
	return nil
}
