package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
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
func shareLink(base string, r *http.Request, token string) string {
	if base != "" {
		return base + "/share/" + token
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host + "/share/" + token
}
func (a *App) describeShare(ctx context.Context, r *http.Request, data pages.ShareDialogData, row queries.Share) pages.ShareDialogData {
	data.Link = shareLink(a.Config.PublicURL, r, row.Token)
	data.Protected = row.PasswordHash.Valid
	if row.ExpiresAt.Valid {
		data.Expires = journalTimestamp(session.FromContext(ctx).Prefs, row.ExpiresAt.Time)
		data.Expired = !row.ExpiresAt.Time.After(time.Now())
	}
	return data
}

type shareSubject struct {
	owner ulid.ULID
	id    ulid.ULID
	entry ulid.ULID
}
type shareGrant struct {
	id       ulid.ULID
	token    string
	password sql.NullString
	expires  sql.NullTime
}
type shareKind struct {
	what      string
	heading   string
	blurb     string
	action    func(s shareSubject) string
	fromQuery func(r *http.Request) (shareSubject, bool)
	fromPath  func(w http.ResponseWriter, r *http.Request) (shareSubject, bool)
	get       func(ctx context.Context, q *queries.Queries, s shareSubject) (queries.Share, error)
	insert    func(ctx context.Context, q *queries.Queries, s shareSubject, g shareGrant) error
	delete    func(ctx context.Context, q *queries.Queries, s shareSubject) (sql.Result, error)
}

func (k shareKind) dialog(s shareSubject) pages.ShareDialogData {
	return pages.ShareDialogData{Heading: k.heading, Blurb: k.blurb, Action: k.action(s)}
}
func (a *App) shareDialog(ctx context.Context, r *http.Request, k shareKind, s shareSubject) (pages.ShareDialogData, error) {
	data := k.dialog(s)
	row, err := k.get(ctx, a.Queries, s)
	if errors.Is(err, sql.ErrNoRows) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	return a.describeShare(ctx, r, data, row), nil
}
func (a *App) shareFragment(w http.ResponseWriter, r *http.Request, k shareKind) {
	ctx := r.Context()
	s, ok := k.fromQuery(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.owner = session.FromContext(ctx).UserID
	data, err := a.shareDialog(ctx, r, k, s)
	if err != nil {
		slog.Error("Failed to load a share", "kind", k.what, "error", err)
		htmx.ServerError(w)
		return
	}
	renderFragment(w, r, pages.ShareDialog(data))
}
func (a *App) createShare(w http.ResponseWriter, r *http.Request, k shareKind) {
	ctx := r.Context()
	s, ok := k.fromPath(w, r)
	if !ok {
		return
	}
	s.owner = session.FromContext(ctx).UserID
	if !parsePanelForm(w, r, pages.ShareDialogPanel) {
		return
	}
	input, problems := buildShareInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.ShareDialogPanel, problems)
		return
	}
	token, err := share.NewToken()
	if err != nil {
		slog.Error("Failed to mint a share token", "error", err)
		htmx.ServerError(w)
		return
	}
	g := shareGrant{id: ulid.Make(), token: token}
	if input.Password != "" {
		hash, err := share.HashPassword(input.Password)
		if err != nil {
			slog.Error("Failed to hash a share password", "error", err)
			htmx.ServerError(w)
			return
		}
		g.password = sql.NullString{String: hash, Valid: true}
	}
	if input.Days > 0 {
		g.expires = sql.NullTime{
			Time:  time.Now().Add(time.Duration(input.Days) * 24 * time.Hour),
			Valid: true,
		}
	}
	if err := k.insert(ctx, a.Queries, s, g); err != nil {
		if data, readErr := a.shareDialog(ctx, r, k, s); readErr == nil && data.Link != "" {
			renderFragment(w, r, pages.ShareDialog(data))
			return
		}
		slog.Error("Failed to create a share", "kind", k.what, "error", err)
		htmx.ServerError(w)
		return
	}
	data, err := a.shareDialog(ctx, r, k, s)
	if err != nil {
		slog.Error("Failed to load a share after creating it", "kind", k.what, "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, k.what)
		return
	}
	htmx.Toast(w, "Share link created.")
	renderFragment(w, r, pages.ShareDialog(data))
}
func (a *App) revokeShare(w http.ResponseWriter, r *http.Request, k shareKind) {
	ctx := r.Context()
	s, ok := k.fromPath(w, r)
	if !ok {
		return
	}
	s.owner = session.FromContext(ctx).UserID
	result, err := k.delete(ctx, a.Queries, s)
	if err != nil {
		slog.Error("Failed to revoke a share", "kind", k.what, "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}
	htmx.Toast(w, "Share link revoked.")
	renderFragment(w, r, pages.ShareDialog(k.dialog(s)))
}
