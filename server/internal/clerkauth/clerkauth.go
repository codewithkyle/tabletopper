


package clerkauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

type Client struct {
	jwks  *jwks.Client
	users *user.Client
}




func New(secretKey string) *Client {
	cfg := &clerk.ClientConfig{}
	cfg.Key = clerk.String(secretKey)
	return &Client{
		jwks:  jwks.NewClient(cfg),
		users: user.NewClient(cfg),
	}
}






const FallbackUsername = "Adventurer"


type Identity struct {
	ClerkID string
	
	
	
	
	Username string
	
	
	
	ImageURL string
}



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

	id := Identity{ClerkID: u.ID, Username: displayName(u)}
	if u.HasImage && u.ImageURL != nil {
		id.ImageURL = *u.ImageURL
	}
	return id, nil
}

















func displayName(u *clerk.User) string {
	if u.Username != nil && strings.TrimSpace(*u.Username) != "" {
		return strings.TrimSpace(*u.Username)
	}

	full := strings.TrimSpace(strings.TrimSpace(deref(u.FirstName)) + " " + strings.TrimSpace(deref(u.LastName)))
	if full != "" {
		return full
	}

	if local := emailLocalPart(primaryEmail(u)); local != "" {
		return local
	}

	return FallbackUsername
}



func primaryEmail(u *clerk.User) string {
	var first string

	for _, address := range u.EmailAddresses {
		if address == nil || address.EmailAddress == "" {
			continue
		}
		if u.PrimaryEmailAddressID != nil && address.ID == *u.PrimaryEmailAddressID {
			return address.EmailAddress
		}
		if first == "" {
			first = address.EmailAddress
		}
	}

	return first
}



func emailLocalPart(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return strings.TrimSpace(address)
	}

	return strings.TrimSpace(address[:at])
}


func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
