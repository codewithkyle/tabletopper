package session

import "context"

type contextKey struct{}

// NewContext returns a copy of ctx carrying s.
func NewContext(ctx context.Context, s UserSession) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext returns the session stashed by the auth middleware. Handlers
// behind RequireSession always get a real session; behind OptionalSession the
// zero value means "logged out".
func FromContext(ctx context.Context) UserSession {
	s, _ := ctx.Value(contextKey{}).(UserSession)
	return s
}
