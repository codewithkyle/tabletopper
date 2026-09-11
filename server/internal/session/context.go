package session

import "context"

type contextKey struct{}


func NewContext(ctx context.Context, s UserSession) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}




func FromContext(ctx context.Context) UserSession {
	s, _ := ctx.Value(contextKey{}).(UserSession)
	return s
}
