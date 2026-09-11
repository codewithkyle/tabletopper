package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/markdown"
	"tabletopper/internal/prefs"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/snippet"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)





























const (
	journalTitleLimit    = 255
	journalBodyLimit     = 262144
	journalSearchLimit   = 255
	journalSnippetRadius = 60
)

func (a *App) CharacterJournalPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	character, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	
	
	
	entries, err := a.journalEntries(ctx, characterID, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load journal entries", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.EditCharacterJournal(pages.JournalPageData{
		CharacterID: characterID.String(),
		Header:      characterHeader(character),
		Entries:     entries,
	}))
}























func (a *App) JournalEntriesFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	params := r.URL.Query()

	characterID, err := ulid.Parse(params.Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	
	
	
	term := strings.TrimSpace(params.Get("q"))
	if len([]rune(term)) > journalSearchLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	entries, err := a.journalEntries(ctx, characterID, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to search journal entries", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.JournalEntriesFragment(pages.JournalPageData{
		CharacterID: characterID.String(),
		Entries:     entries,
		Query:       term,
	}))
}








func (a *App) CharacterJournalEntryPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	entryID, err := ulid.Parse(r.PathValue("entryId"))
	if err != nil {
		redirectToJournal(w, r)
		return
	}

	character, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	entry, err := a.Queries.GetJournalEntry(ctx, queries.GetJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirectToJournal(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load journal entry", "error", err)
		redirectToError(w, r)
		return
	}

	
	
	
	
	
	
	
	
	
	
	
	
	
	w.Header().Set("Content-Security-Policy", "img-src 'self'")

	render(w, r, pages.EditCharacterJournalEntry(pages.JournalEntryPageData{
		CharacterID: characterID.String(),
		Header:      characterHeader(character),
		EntryID:     entry.ID.String(),
		Title:       entry.Title,
		Body:        entry.Body,
	}))
}










func (a *App) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	entryID := ulid.Make()
	
	
	
	
	result, err := a.Queries.InsertJournalEntry(ctx, queries.InsertJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to create journal entry", "error", err)
		redirectToError(w, r)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		redirect(w, r, "/characters")
		return
	}

	redirect(w, r, "/characters/"+characterID.String()+"/edit/journal/"+entryID.String())
}



func (a *App) SaveJournalEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
	if !ok {
		return
	}

	if !parsePanelForm(w, r, pages.JournalEntryPanel) {
		return
	}

	input, problems := buildJournalInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.JournalEntryPanel, problems)
		return
	}

	result, err := a.Queries.UpdateJournalEntry(ctx, queries.UpdateJournalEntryParams{
		Title:       input.Title,
		Body:        input.Body,
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	
	
	
	
	
	
	finishJournalEntry(w, r, result, err, r.PostFormValue("announce") != "", func() {
		a.reconcileJournalImages(ctx, characterID, entryID, sess.UserID, input.Body)
	})
}











func (a *App) DeleteJournalEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
	if !ok {
		return
	}

	
	
	
	
	
	
	
	
	
	if _, err := a.Queries.DeleteJournalShare(ctx, queries.DeleteJournalShareParams{
		EntryID:     entryID,
		CharacterID: &characterID,
		OwnerID:     sess.UserID,
	}); err != nil {
		slog.Error("Failed to revoke journal share", "error", err)
		htmx.ServerError(w)
		return
	}

	
	
	
	
	
	
	err := a.Queries.DetachJournalImages(ctx, queries.DetachJournalImagesParams{
		JournalID: &entryID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to detach journal images", "error", err)
		htmx.ServerError(w)
		return
	}

	result, err := a.Queries.DeleteJournalEntry(ctx, queries.DeleteJournalEntryParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete journal entry", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "journal entry")
		return
	}

	htmx.Toast(w, "Entry deleted.")
}























func finishJournalEntry(w http.ResponseWriter, r *http.Request, result sql.Result, err error, announce bool, saved func()) {
	
	
	
	if !savedRow(w, pages.JournalEntryPanel, "journal entry", result, err) {
		return
	}

	saved()

	if announce {
		htmx.Toast(w, "Entry saved.")
	}

	renderPanelBlock(w, r, pages.JournalEntryPanel, nil)
}













func (a *App) JournalLinkFragment(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.JournalLinkFragment())
}

type journalInput struct {
	Title string
	Body  string
}








func buildJournalInput(r *http.Request) (journalInput, []string) {
	var problems []string

	title := strings.TrimSpace(r.PostFormValue("title"))
	if len([]rune(title)) > journalTitleLimit {
		problems = append(problems, "Title must be 255 characters or fewer.")
	}

	body := r.PostFormValue("body")
	if len(body) > journalBodyLimit {
		problems = append(problems, "This entry is too long to save. Split it into two.")
	}

	return journalInput{Title: title, Body: body}, problems
}

func journalEntryID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	entryID, err := ulid.Parse(r.PathValue("entryId"))
	if err != nil {
		htmx.NotFound(w, "journal entry")
		return ulid.ULID{}, false
	}

	return entryID, true
}





func redirectToJournal(w http.ResponseWriter, r *http.Request) {
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	redirect(w, r, "/characters/"+characterID.String()+"/edit/journal")
}





























func (a *App) journalEntries(ctx context.Context, characterID, ownerID ulid.ULID, term string) ([]pages.JournalEntry, error) {
	p := session.FromContext(ctx).Prefs

	if term == "" {
		rows, err := a.Queries.ListCharacterJournals(ctx, queries.ListCharacterJournalsParams{
			CharacterID: characterID,
			OwnerID:     ownerID,
		})
		if err != nil {
			return nil, err
		}

		entries := make([]pages.JournalEntry, 0, len(rows))
		for _, row := range rows {
			entries = append(entries, journalPageEntry(p, row.ID, row.Title, row.CreatedAt, row.UpdatedAt))
		}

		return entries, nil
	}

	rows, err := a.Queries.SearchCharacterJournals(ctx, queries.SearchCharacterJournalsParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
		Term:        journalSearchPattern(term),
	})
	if err != nil {
		return nil, err
	}

	entries := make([]pages.JournalEntry, 0, len(rows))
	for _, row := range rows {
		entry := journalPageEntry(p, row.ID, row.Title, row.CreatedAt, row.UpdatedAt)

		hit, found := snippet.Find(markdown.PlainText(row.Body), term, journalSnippetRadius)
		if found {
			entry.Snippet = pages.JournalSnippet{Before: hit.Before, Match: hit.Match, After: hit.After}
		}
		if !found && !snippet.Contains(row.Title, term) {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}









var journalSearchWildcards = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)





func journalSearchPattern(term string) string {
	return "%" + journalSearchWildcards.Replace(term) + "%"
}

func journalPageEntry(p prefs.Preferences, id ulid.ULID, title string, created, updated time.Time) pages.JournalEntry {
	return pages.JournalEntry{
		ID:      id.String(),
		Title:   title,
		Created: journalTimestamp(p, created),
		Updated: journalTimestamp(p, updated),
	}
}











func journalTimestamp(p prefs.Preferences, at time.Time) pages.Timestamp {
	iso, text := p.Format(at)

	return pages.Timestamp{ISO: iso, Text: text}
}
