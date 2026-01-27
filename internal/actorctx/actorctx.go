package actorctx

import (
	"context"

	"github.com/geocoder89/eventhub/internal/http/middlewares"
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, middlewares.KeyUserID, userID)
}

func UserIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(middlewares.KeyUserID).(string)

	return v, ok && v != ""
}
