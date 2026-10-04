package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"
)

type shareInput struct {
	Days     int
	Password string
}

func buildShareInput(r *http.Request) (shareInput, []string) {
	var input shareInput
	var problems []string
	if r.PostFormValue("expiry") != "" {
		days, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("days")))
		if err != nil || days < pages.ShareMinDays || days > pages.ShareMaxDays {
			problems = append(problems, fmt.Sprintf(
				"Choose between %d and %d days, or turn the expiry off.", pages.ShareMinDays, pages.ShareMaxDays))
		} else {
			input.Days = days
		}
	}
	if r.PostFormValue("protect") != "" {
		password := r.PostFormValue("password")
		switch {
		case len(password) < share.PasswordMin:
			problems = append(problems, fmt.Sprintf("A password must be at least %d characters.", share.PasswordMin))
		case len(password) > share.PasswordMax:
			problems = append(problems, fmt.Sprintf("A password must be %d characters or fewer.", share.PasswordMax))
		default:
			input.Password = password
		}
	}
	return input, problems
}
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
func describeShare(ctx context.Context, r *http.Request, data pages.ShareDialogData, row queries.Share) pages.ShareDialogData {
	data.Link = shareLink(r, row.Token)
	data.Protected = row.PasswordHash.Valid
	if row.ExpiresAt.Valid {
		data.Expires = journalTimestamp(session.FromContext(ctx).Prefs, row.ExpiresAt.Time)
		data.Expired = !row.ExpiresAt.Time.After(time.Now())
	}
	return data
}
