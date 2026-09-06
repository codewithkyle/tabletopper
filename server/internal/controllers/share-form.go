package controllers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"
)

// What the two share dialogs have in common, which is everything except the
// thing being shared: the expiry and the password the form collects, and the
// absolute URL the created link is shown as.
//
// It is its own file because both callers are peers. Leaving it in
// journal-share.go and calling it from character-share.go would have said the
// journal owns the form and the character borrows it, which is not true of a
// single line in here -- neither file's subject appears in it.

const (
	// shareMinDays and shareMaxDays bound the expiry the form collects. The
	// floor is a day because the box counts in days and zero of them is a link
	// that is dead before it is pasted; the ceiling is a year because a share
	// meant to outlast one is a share with no expiry, which the toggle already
	// offers.
	shareMinDays = 1
	shareMaxDays = 365
)

// shareInput is a validated create request: how many days until the link dies,
// zero for never, and the password, empty for none. Both toggles collapse into
// their field's zero value, so nothing downstream has to consult a flag and a
// value that could disagree.
type shareInput struct {
	Days     int
	Password string
}

// buildShareInput reads the dialog's two toggles and the field under each.
//
// A TOGGLE THAT IS OFF DISCARDS THE FIELD BESIDE IT WITHOUT LOOKING AT IT. The
// days box always posts a value -- it holds a default so that flipping the
// toggle is a complete answer -- so reading it regardless would give every link
// an expiry nobody asked for.
func buildShareInput(r *http.Request) (shareInput, []string) {
	var input shareInput
	var problems []string

	if r.PostFormValue("expiry") != "" {
		days, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("days")))
		if err != nil || days < shareMinDays || days > shareMaxDays {
			problems = append(problems, "Choose between 1 and 365 days, or turn the expiry off.")
		} else {
			input.Days = days
		}
	}

	if r.PostFormValue("protect") != "" {
		// NOT TRIMMED. A password is a secret rather than a name, and a space
		// at either end of it is a character the person who chose it typed --
		// trimming here would silently store something they could not then
		// type back in.
		password := r.PostFormValue("password")
		switch {
		case len(password) < share.PasswordMin:
			problems = append(problems, "A password must be at least 6 characters.")
		case len(password) > share.PasswordMax:
			problems = append(problems, "A password must be 72 characters or fewer.")
		default:
			input.Password = password
		}
	}

	return input, problems
}

// shareLink is the absolute URL of a share, which is what the dialog puts in
// front of someone to copy. It is the one URL in the app built with a scheme
// and a host, because it is the only one that leaves the app: everything else
// is a path the browser resolves against the page it is already on.
//
// THE HOST COMES FROM THE REQUEST, which is the only thing that knows it. There
// is no configured base URL, and adding one would be a variable to keep in step
// with wherever this is deployed for a value the request already carries.
//
// The scheme is the header the proxy in front sets, and only two values are
// accepted from it -- it is a client-supplied header when nothing is in front,
// and the worst a forged one can do is make a copied link say https where the
// deployment is plain http. Falling back to the connection is what makes local
// development produce http://localhost:3000 without configuring anything.
func shareLink(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}

	return scheme + "://" + r.Host + "/share/" + token
}

// describeShare adds what a live link says about itself to a dialog that
// already knows which share it is: the URL to copy, whether a password stands
// in front of it, and when it stops working.
//
// IT TAKES THE ROW RATHER THAN THE PARAMS THAT MADE IT, and both callers read
// the row back after inserting for exactly that reason: the dialog then has one
// definition of what it shows, and an insert that matched nothing -- a thing
// that is not this user's -- is found as an absent link rather than reported as
// a link that was never created.
//
// THE EXPIRY COMPARISON IS HERE AND NOT IN THE STATEMENT. Neither owner read
// filters on it, because an expired share is still a row its owner has to be
// shown and offered the chance to revoke; so the comparison those queries skip
// is made once, here, for the sentence the dialog prints.
func describeShare(ctx context.Context, r *http.Request, data pages.ShareDialogData, row queries.Share) pages.ShareDialogData {
	data.Link = shareLink(r, row.Token)
	data.Protected = row.PasswordHash.Valid

	if row.ExpiresAt.Valid {
		data.Expires = journalTimestamp(session.FromContext(ctx).Prefs, row.ExpiresAt.Time)
		data.Expired = !row.ExpiresAt.Time.After(time.Now())
	}

	return data
}
