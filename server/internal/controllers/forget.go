package controllers

import (
	"context"

	"github.com/oklog/ulid/v2"
)

func (a *App) forgetCharacter(ctx context.Context, character ulid.ULID) {
	if a.Hub != nil {
		a.Hub.ForgetCharacter(ctx, character)
	}
}
func (a *App) forgetMonster(ctx context.Context, monster ulid.ULID, image *ulid.ULID) {
	if a.Hub == nil {
		return
	}
	a.Hub.ForgetMonster(ctx, monster)
	if image != nil {
		a.Hub.ForgetAsset(ctx, *image)
	}
}
func (a *App) forgetAsset(ctx context.Context, asset ulid.ULID) {
	if a.Hub != nil {
		a.Hub.ForgetAsset(ctx, asset)
	}
}
