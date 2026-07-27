package authctx

import (
	"context"
	"errors"
)

type ctxKey int

const userIDKey ctxKey = 1

var ErrNoUser = errors.New("authentication required")

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserID(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDKey).(int64)
	return v, ok && v > 0
}

func RequireUserID(ctx context.Context) (int64, error) {
	id, ok := UserID(ctx)
	if !ok {
		return 0, ErrNoUser
	}
	return id, nil
}
