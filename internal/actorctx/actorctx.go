package actorctx

import (
	"context"
)

type key string

const (
	userIDKey    key = "user_id"
	requestIDKey key = "request_id"
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)

	return v, ok && v != ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(requestIDKey).(string)

	return v, ok && v != ""
}
