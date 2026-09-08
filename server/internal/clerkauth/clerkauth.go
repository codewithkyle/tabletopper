// Package clerkauth is the seam between this app and Clerk: verify the
// session token Clerk's frontend set, and read the user it names. It is the
// only importer of the Clerk SDK, so a Clerk API change lands here alone.
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

// FallbackUsername is what an account is called when Clerk knows no name for it
// at all: no username, no first or last name, and no email address. Nothing
// reaches it in practice -- every provider supplies at least one of those -- and
// it exists so that "an account has a name" is true everywhere rather than
// nearly everywhere, because the alternative is a blank line in a player list.
const FallbackUsername = "Adventurer"

// Identity is what the app keeps of a Clerk user.
type Identity struct {
	ClerkID string
	// Username is never empty, and is only sometimes a Clerk username. A
	// sign-up through an OAuth provider need not have chosen one -- Google
	// sign-ups have no username at all -- so this is the first of several
	// things Clerk might know, resolved by displayName below.
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

	id := Identity{ClerkID: u.ID, Username: displayName(u)}
	if u.HasImage && u.ImageURL != nil {
		id.ImageURL = *u.ImageURL
	}
	return id, nil
}

// displayName is the first name Clerk has for this account, and it is a chain
// rather than one field because only some providers fill in each link.
//
// A USERNAME IS OPTIONAL IN CLERK, AND OAUTH SIGN-UPS DO NOT HAVE ONE. Discord
// hands one over, so the field was enough while that was the only way in;
// Google hands over a first and last name and an email address and no username,
// which arrived here as an empty string and was written into the users row as
// an account with no name. That is what this exists to stop.
//
// The email is the last resort before the constant, and only its local part is
// taken -- somebody's full address is not a name to print beside their pawn,
// and the domain is the half that says where they work.
//
// The result is only ever a seed. It is written once, at sign-up, and the
// account renames itself from the settings dialog after that; a later login
// does not come back and overwrite the answer.
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

// primaryEmail is the address Clerk marks primary, or the first one it has if
// the account has addresses but no primary among them.
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

// emailLocalPart is everything before the @, trimmed. An address with nothing
// before the @ is not one, and returns empty so the chain falls through.
func emailLocalPart(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return strings.TrimSpace(address)
	}

	return strings.TrimSpace(address[:at])
}

// deref reads a *string Clerk may have left nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
