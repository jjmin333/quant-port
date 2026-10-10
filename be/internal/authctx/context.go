package authctx

import (
	"context"
	"errors"
	"sync/atomic"
)

type ctxKey int

const (
	userIDKey ctxKey = 1
	probeKey  ctxKey = 2
)

var ErrNoUser = errors.New("authentication required")

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserID(ctx context.Context) (int64, bool) {
	if p, _ := ctx.Value(probeKey).(*IdentityProbe); p != nil {
		p.consulted.Store(true)
	}
	v, ok := ctx.Value(userIDKey).(int64)
	return v, ok && v > 0
}

/*
IdentityProbe — 이 요청을 처리하는 동안 **누가 묻는지를 한 번이라도 봤는가.**

공유 캐시(Cloudflare 엣지)에 응답을 맡겨도 되는지는 「이 응답이 요청자와 무관한가」로
정해진다. 핸들러 목록을 손으로 관리하면 언젠가 누가 공개 경로에 관심·보유 표시를
덧씌우고(`/quotes/*`가 이미 그렇다) 남의 표시가 캐시돼 퍼진다. 그래서 규칙을
코드가 지키게 한다: 처리 중 `UserID`가 한 번이라도 불렸으면 — **비로그인이라 답이
「없음」이었어도** — 그 경로는 요청자에 따라 달라질 수 있는 경로다.

고루틴으로 퍼지는 서비스가 있어 원자값이다.
*/
type IdentityProbe struct {
	consulted atomic.Bool
}

// WithIdentityProbe attaches a fresh probe; read it back after the handler ran.
func WithIdentityProbe(ctx context.Context) (context.Context, *IdentityProbe) {
	p := &IdentityProbe{}
	return context.WithValue(ctx, probeKey, p), p
}

// Consulted reports whether anything asked for the caller's identity.
func (p *IdentityProbe) Consulted() bool {
	return p != nil && p.consulted.Load()
}

func RequireUserID(ctx context.Context) (int64, error) {
	id, ok := UserID(ctx)
	if !ok {
		return 0, ErrNoUser
	}
	return id, nil
}

const adminKey ctxKey = 3

// WithAdmin marks the caller as an admin. Only middleware.MarkAdmin sets it,
// and only on the routes that let an admin act on someone else's content.
func WithAdmin(ctx context.Context) context.Context {
	return context.WithValue(ctx, adminKey, true)
}

// IsAdmin is false unless MarkAdmin ran on this route and the caller is an admin.
func IsAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(adminKey).(bool)
	return v
}
