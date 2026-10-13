package controllers

import (
	"context"

	"github.com/oklog/ulid/v2"
)

func (a *App) forgetMonster(ctx context.Context, monster ulid.ULID, image *ulid.ULID) {
	a.Hub.ForgetMonster(ctx, monster)
	if image != nil {
		a.Hub.ForgetAsset(ctx, *image)
	}
}
