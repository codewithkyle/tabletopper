// Package clerkauth is the seam between this app and Clerk: verify the
// session token Clerk's frontend set, and read the user it names. It is the
// only importer of the Clerk SDK, so a Clerk API change lands here alone.
package clerkauth

import (
	"context"
	"fmt"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

type Client struct {
	jwks  *jwks.Client
	users *user.Client
}

// New builds a client bound to secretKey. The SDK also offers a package-level
// key; this deliberately does not use it, so two Clients could coexist and a
// test can pass its own.
func New(secretKey string) *Client {
	cfg := &clerk.ClientConfig{}
	cfg.Key = clerk.String(secretKey)
	return &Client{
		jwks:  jwks.NewClient(cfg),
		users: user.NewClient(cfg),
	}
}

// Identity is what the app keeps of a Clerk user.
type Identity struct {
	ClerkID string
	// Username may be empty: a sign-up through an OAuth provider need not
	// have chosen one.
	Username string
	// ImageURL is empty when the user has no picture of their own. Clerk
	// always supplies a URL, but it is a generated placeholder unless
	// HasImage says otherwise, and we have our own placeholder.
	ImageURL string
}

// Authenticate verifies token and reads the user it names. The JWKS is
// fetched per call; this runs once per login, not per request.
func (c *Client) Authenticate(ctx context.Context, token string) (Identity, error) {
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{
		Token:      token,
		JWKSClient: c.jwks,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("clerk: verify token: %w", err)
	}

	u, err := c.users.Get(ctx, claims.Subject)
	if err != nil {
		return Identity{}, fmt.Errorf("clerk: read user %s: %w", claims.Subject, err)
	}

	id := Identity{ClerkID: u.ID}
	if u.Username != nil {
		id.Username = *u.Username
	}
	if u.HasImage && u.ImageURL != nil {
		id.ImageURL = *u.ImageURL
	}
	return id, nil
}
