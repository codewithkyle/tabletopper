package clerkauth
import (
	"testing"
	"github.com/clerk/clerk-sdk-go/v2"
)
func str(s string) *string { return &s }
func email(id string, address string) *clerk.EmailAddress {
	return &clerk.EmailAddress{ID: id, EmailAddress: address}
}
func TestADisplayNameIsResolvedFromWhateverClerkKnows(t *testing.T) {
	tests := map[string]struct {
		user *clerk.User
		want string
	}{
		"a username wins": {
			user: &clerk.User{
				Username:  str("kyle"),
				FirstName: str("Kyle"),
				LastName:  str("Andrews"),
			},
			want: "kyle",
		},
		"a Google sign-up falls back to the full name": {
			user: &clerk.User{
				FirstName:             str("Kyle"),
				LastName:              str("Andrews"),
				PrimaryEmailAddressID: str("idn_1"),
				EmailAddresses:        []*clerk.EmailAddress{email("idn_1", "someone@example.com")},
			},
			want: "Kyle Andrews",
		},
		"a first name alone is a name": {
			user: &clerk.User{FirstName: str("Kyle")},
			want: "Kyle",
		},
		"a last name alone is a name": {
			user: &clerk.User{LastName: str("Andrews")},
			want: "Andrews",
		},
		"no name at all falls back to the email's local part": {
			user: &clerk.User{
				PrimaryEmailAddressID: str("idn_2"),
				EmailAddresses: []*clerk.EmailAddress{
					email("idn_1", "old@example.com"),
					email("idn_2", "kyle.andrews@example.com"),
				},
			},
			want: "kyle.andrews",
		},
		"an account with addresses but no primary takes the first": {
			user: &clerk.User{
				EmailAddresses: []*clerk.EmailAddress{email("idn_1", "first@example.com")},
			},
			want: "first",
		},
		"whitespace is not a username": {
			user: &clerk.User{Username: str("   "), FirstName: str("Kyle")},
			want: "Kyle",
		},
		"whitespace is not a name either": {
			user: &clerk.User{
				FirstName:      str("  "),
				LastName:       str("  "),
				EmailAddresses: []*clerk.EmailAddress{email("idn_1", "kyle@example.com")},
			},
			want: "kyle",
		},
		"an address with nothing before the @ is not a name": {
			user: &clerk.User{
				EmailAddresses: []*clerk.EmailAddress{email("idn_1", "@example.com")},
			},
			want: FallbackUsername,
		},
		"an account Clerk knows nothing about still gets a name": {
			user: &clerk.User{},
			want: FallbackUsername,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := displayName(tt.user); got != tt.want {
				t.Errorf("displayName = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestADisplayNameIsNeverEmpty(t *testing.T) {
	for name, user := range map[string]*clerk.User{
		"nothing":            {},
		"nil everywhere":     {Username: nil, FirstName: nil, LastName: nil},
		"an empty username":  {Username: str("")},
		"an empty address":   {EmailAddresses: []*clerk.EmailAddress{email("idn_1", "")}},
		"a nil address slot": {EmailAddresses: []*clerk.EmailAddress{nil}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := displayName(user); got == "" {
				t.Error("displayName returned the empty string")
			}
		})
	}
}
