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

const (
	shareMinDays = 1
	shareMaxDays = 365
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
		if err != nil || days < shareMinDays || days > shareMaxDays {
			problems = append(problems, "Choose between 1 and 365 days, or turn the expiry off.")
		} else {
			input.Days = days
		}
	}
	if r.PostFormValue("protect") != "" {
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
