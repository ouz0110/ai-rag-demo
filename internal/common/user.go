package common

import (
	"context"
)

type userContextKey struct{}

type User struct {
	Openid string
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

func UserFromContext(ctx context.Context) (bool, User) {
	u, ok := ctx.Value(userContextKey{}).(User)
	return ok, u
}
