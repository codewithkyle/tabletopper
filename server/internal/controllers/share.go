package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/markdown"
	"tabletopper/internal/queries"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// The reader's half of sharing: four routes in front of no session at all, and
// the token machinery both kinds of share go through. journal-share.go and
// character-share.go are the owners' halves, share-character.go is what a shared
// sheet renders as, and none of them shares anything with this file but the row.
//
// THE GATE, THE HEADERS AND THE MISS ARE HERE ONCE FOR BOTH KINDS. A password
// answered in one place cannot be forgotten by the next thing made shareable,
// which is the whole reason SharePage branches at the bottom rather than the
// top.
//
// THESE ARE THE ONLY HANDLERS IN THE APP THAT RUN FOR SOMEBODY THE APP HAS
// NEVER HEARD OF, and every one of them is written from that. There is no
// session to read, so the token in the path is the entire authorisation; there
// is no page to redirect to, so a miss is the same flat answer whatever went
// wrong; and there is no next request to correct a mistake in, so anything not
// deliberately exposed here is not exposed at all.
//
// THE TOKEN IS NEVER USED TO FIND ANYTHING BUT THE SHARE ROW. Every other id
// these handlers touch -- the entry, the character, the owner -- comes off that
// row rather than out of the URL, so the whole of what a link can reach is
// decided by the row it names and cannot be widened by editing the address bar.
// The one exception is the asset id in the image path, and it is checked
// against the entry the row named before a single byte is served.
//
// A MISS IS ALWAYS THE SAME MISS. An expired share, a revoked one, a token that
// was never issued and an entry deleted since it was shared all answer with the
// one page, because telling them apart would tell whoever is guessing which
// half of the guess was right.

// SharePage is the one URL both kinds of share are read at: a journal entry, a
// character sheet, or the password gate in front of either.
//
// NOTHING IS READ UNTIL THE PASSWORD IS ANSWERED. GetShareByToken selects no
// journal and no character columns, and the branch below is the first statement
// past the gate -- so a locked share is one the process never had the contents
// of, rather than one it held in memory with a branch keeping it off the wire.
// That is also why the gate is here rather than inside each branch: a question
// asked in one place cannot be forgotten by the next kind of share added.
//
// THE TYPE COMES OFF THE ROW, LIKE EVERY OTHER ID THESE HANDLERS USE. There is
// nothing in the URL that says which of the two this is, so there is nothing in
// the URL to edit into asking for the other -- a token names a row and the row
// decides what it opens.
func (a *App) SharePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.PathValue("token")
	if !share.ValidToken(token) {
		shareUnavailable(w, r)
		return
	}

	grant, err := a.Queries.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load share", "error", err)
		redirectToError(w, r)
		return
	}

	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareHeaders(w)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{Action: "/share/" + token}))
		return
	}

	switch grant.ResourceType {
	case queries.SharesResourceTypeJournal:
		a.sharedJournalEntry(w, r, token, grant)
	case queries.SharesResourceTypeCharacter:
		a.sharedCharacterSheet(w, r, token, grant)
	default:
		// An enum member nothing here renders, which today is unreachable and
		// is written down anyway: the column can gain a member in a migration
		// without this file being opened, and the failure that would cause is
		// a blank 200 rather than anything that announces itself. A dead link
		// is the honest answer -- the row exists, and there is nothing this
		// build can open it as.
		slog.Error("Share names a resource type this build cannot render", "type", grant.ResourceType)
		shareUnavailable(w, r)
	}
}

// sharedJournalEntry is the page one entry's link opens. It runs past the gate
// in SharePage and takes the grant rather than re-reading it, so the password is
// answered exactly once per request.
func (a *App) sharedJournalEntry(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	ctx := r.Context()

	entry, err := a.Queries.GetSharedJournalEntry(ctx, queries.GetSharedJournalEntryParams{
		EntryID:     grant.ResourceID,
		CharacterID: grant.CharacterID,
		OwnerID:     grant.OwnerID,
	})
	// The entry was deleted after the link went out. The share row outlives it
	// -- nothing about deleting an entry reaches this table until the entry
	// delete's own statement runs -- so this is a real miss rather than an
	// impossible one, and it reads as a dead link, which is what it is.
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared journal entry", "error", err)
		redirectToError(w, r)
		return
	}

	body, err := markdown.Render(entry.Body, shareImageSource(token, grant.CharacterID, grant.ResourceID))
	if err != nil {
		slog.Error("Failed to render shared journal entry", "error", err)
		redirectToError(w, r)
		return
	}

	character := pages.SharedCharacter{
		Name:    entry.Name,
		Level:   strconv.Itoa(int(entry.Level)),
		Classes: fallbackString(nullStringValue(entry.Classes), "Class Unknown"),
		Race:    fallbackString(nullStringValue(entry.Race), "Unknown Lineage"),
	}
	if entry.AssetID != nil {
		character.Avatar = "/share/" + token + "/avatar"
	}

	shareHeaders(w)
	render(w, r, pages.SharedJournalEntry(pages.SharedJournalData{
		Character: character,
		Title:     entry.Title,
		Body:      body,
	}))
}

// UnlockShare takes the password and, if it is right, leaves the browser
// holding the proof.
//
// IT IS A PLAIN FORM POST ANSWERED WITH A 303, not htmx, and the share layout
// loads no JavaScript at all. A gate that needs a script to open is a gate that
// does not open for a reader whose network ate one file.
//
// THE WRONG PASSWORD IS 401 AND SAYS NOTHING ELSE. It does not say whether the
// share exists, when it expires or whose it is, because all three are behind
// the question being asked. There is no attempt counter either: bcrypt at the
// default cost is the rate limit -- see share.HashPassword.
func (a *App) UnlockShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.PathValue("token")
	if !share.ValidToken(token) {
		shareUnavailable(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		shareUnavailable(w, r)
		return
	}

	grant, err := a.Queries.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load share for unlock", "error", err)
		redirectToError(w, r)
		return
	}

	// A share with no password has nothing to unlock, and posting to one is a
	// stale form rather than an attack. Sending the reader to the entry is the
	// answer they were after.
	if !grant.PasswordHash.Valid {
		redirect(w, r, "/share/"+token)
		return
	}

	if !share.PasswordMatches(grant.PasswordHash.String, r.PostFormValue("password")) {
		shareHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{
			Action:  "/share/" + token,
			Problem: "That password is not right.",
		}))
		return
	}

	// Secure follows the deployment rather than the request: development runs
	// on plain http, where a Secure cookie is set and never sent back, and the
	// gate would ask the same question forever.
	share.SetUnlocked(w, token, grant.PasswordHash.String, !a.Config.Development())
	redirect(w, r, "/share/"+token)
}

// GetShareImage serves one picture from a shared entry.
//
// THIS ROUTE IS WHY THE PASSWORD IS WORTH ANYTHING. A gate on the page with an
// ungated image route would be a locked door beside an open window: the
// pictures are most of what an entry is, and their URLs are in the markup the
// gate is protecting. So it asks the same question SharePage asks, from the
// same cookie, before it looks at an asset id.
//
// The id in the path is checked against the row rather than trusted:
// GetJournalImage carries the asset, the entry, the character and the owner,
// and the last three come off the share. An asset that is not this entry's --
// another entry's picture, the owner's map, an avatar -- matches nothing.
func (a *App) GetShareImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.PathValue("token")
	assetID, err := ulid.Parse(r.PathValue("assetId"))
	if !share.ValidToken(token) || err != nil {
		http.NotFound(w, r)
		return
	}

	grant, ok := a.shareGrant(w, r, token)
	if !ok {
		return
	}

	// A CHARACTER SHARE HAS NO IMAGES HERE, and it is refused rather than left
	// to miss. Its resource_id is a character id, so GetJournalImage below
	// would match nothing and 404 anyway -- but that is a join happening to
	// come back empty, and "no picture belongs to this share" is a rule this
	// route should state rather than inherit. The sheet's one picture is the
	// avatar, on its own route.
	if grant.ResourceType != queries.SharesResourceTypeJournal {
		http.NotFound(w, r)
		return
	}

	key, err := a.Queries.GetJournalImage(ctx, queries.GetJournalImageParams{
		AssetID:     assetID,
		EntryID:     grant.ResourceID,
		CharacterID: grant.CharacterID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load shared journal image row", "error", err, "assetID", assetID.String())
		}
		http.NotFound(w, r)
		return
	}

	// Cached the way GetJournalImage caches: a journal image is never replaced
	// at its URL, so the bytes behind an id cannot change. private, because
	// whether there is an answer at all depends on a cookie.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	a.streamImage(w, r, key, `"`+assetID.String()+`"`)
}

// GetShareAvatar serves the portrait in the banner.
//
// THE URL NAMES NOTHING, which is the whole of its scoping. A share has one
// character and a character has one avatar, so there is no id to check and
// none to tamper with -- the path cannot be edited into a request for any
// other image the owner holds.
//
// no-cache rather than immutable, unlike the images above: an avatar IS
// replaced at its id -- the upload overwrites the same key -- so the browser
// keeps the bytes and asks first, and the ETag off updated_at is what answers.
func (a *App) GetShareAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.PathValue("token")
	if !share.ValidToken(token) {
		http.NotFound(w, r)
		return
	}

	grant, ok := a.shareGrant(w, r, token)
	if !ok {
		return
	}

	avatar, err := a.Queries.GetSharedCharacterAvatar(ctx, queries.GetSharedCharacterAvatarParams{
		CharacterID: grant.CharacterID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load shared character avatar", "error", err, "characterID", grant.CharacterID.String())
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	a.streamImage(w, r, avatar.FilePath, fmt.Sprintf(`"%s-%d"`, grant.CharacterID, avatar.UpdatedAt.Unix()))
}

// shareGrant is the lookup and the gate for the two image routes, which want
// the same answer to both questions and have the same thing to say when either
// one comes back no.
//
// EVERY REFUSAL IS http.NotFound, INCLUDING THE LOCKED ONE. The caller is an
// <img>, which has nothing to render a 401 into and no form to put a password
// in; and a status that distinguished "wrong link" from "right link, locked"
// would answer, for anyone holding a URL, the only question the password was
// there to keep them from answering.
func (a *App) shareGrant(w http.ResponseWriter, r *http.Request, token string) (queries.GetShareByTokenRow, bool) {
	grant, err := a.Queries.GetShareByToken(r.Context(), token)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load share", "error", err)
		}
		http.NotFound(w, r)
		return queries.GetShareByTokenRow{}, false
	}

	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		http.NotFound(w, r)
		return queries.GetShareByTokenRow{}, false
	}

	return grant, true
}

// shareImageSource maps the image URLs stored in a body onto this share's own,
// and drops everything else. It is the strip half of what the entry page's CSP
// is the backstop for -- see markdown.ImageSource -- and it is exact rather
// than approximate: the prefix names this character and this entry, so another
// entry's picture is refused by the same rule that refuses a tracking pixel.
//
// The remainder has to parse as a ULID, so nothing from a body can be appended
// to the share's path -- a destination ending in `../../elsewhere` is not a
// ULID and never reaches the URL this builds.
func shareImageSource(token string, characterID, entryID ulid.ULID) markdown.ImageSource {
	prefix := journalImagePrefix(characterID, entryID)

	return func(dest string) (string, bool) {
		assetID, found := strings.CutPrefix(dest, prefix)
		if !found {
			return "", false
		}
		if _, err := ulid.Parse(assetID); err != nil {
			return "", false
		}

		return "/share/" + token + "/images/" + assetID, true
	}
}

// shareHeaders is what every shared page carries, and each line answers
// something specific about handing a URL to strangers.
//
// Content-Security-Policy is the entry page's line, for the same reason and as
// the same backstop: foreign images are already stripped when the body renders,
// and this is what refuses one that somehow survived rather than letting a
// reader's browser fetch it.
//
// REFERRER-POLICY IS THE ONE THIS PAGE NEEDS AND THE ENTRY PAGE DOES NOT. The
// token is in the URL, and a link inside the entry sends the URL it was clicked
// from to whoever is on the other end -- so without this, one outbound link in
// a shared entry hands the share itself to a third party's access log.
//
// X-Robots-Tag is the header half of the noindex in the layout: an unlisted
// link a crawler files away is not unlisted, and a fetcher that never parses
// the body sees only this.
//
// Cache-Control is no-store because whether this page has a body at all depends
// on a cookie, and a shared cache holding the answer to the unlocked request
// would serve it to the locked one.
func shareHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "img-src 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")
}

// shareUnavailable is every dead end: a malformed token, a revoked share, an
// expired one, and an entry deleted since the link went out. One page and one
// status for all of them, because the differences between them are exactly
// what somebody guessing at URLs would like to be told.
func shareUnavailable(w http.ResponseWriter, r *http.Request) {
	shareHeaders(w)
	w.WriteHeader(http.StatusNotFound)
	render(w, r, pages.ShareUnavailable())
}
