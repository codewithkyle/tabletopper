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

	"tabletopper/internal/htmx"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/disintegration/imaging"
	"github.com/oklog/ulid/v2"
)

// The manual: the roster's shape for the other thing a GM writes down before a
// session. A page of cards, a one-question dialog that creates one and sends the
// browser to its editor, and a delete that empties everything the monster owns.

// MonstersPage is the whole manual, unfiltered. The search box above the grid
// narrows it through the fragment below, and the page itself never renders a
// filtered list -- a ?q= here would have to come from a bookmark, because
// nothing puts one in the URL.
func (a *App) MonstersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsters, err := a.monsterList(ctx, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load monsters", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.Monsters(pages.MonsterListData{Monsters: monsters}))
}

// MonsterListFragment is the grid under the search box, filtered by ?q=. It is a
// GET returning the same component the page renders, which is what the
// /fragment/ prefix promises.
//
// IT LOADS NOTHING TO CHECK OWNERSHIP, because owner_id is in the query beside
// the term: a manual belonging to somebody else is not addressable from here at
// all -- there is no id in this URL to name one with.
//
// THE SEARCH IS NOT IN THE URL, and hx-push-url is deliberately absent from the
// box, for the reason the journal's search gives: htmx pushes on every swap and
// the swaps are on a debounce, so pushing would file a history entry per pause
// in typing.
//
// An overlong term is a 404 with an empty body rather than an alert. The box
// carries a maxlength, so a term past the column's width came from something
// other than the box, and there is nobody on the other end to tell.
func (a *App) MonsterListFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	// Trimmed, so a term someone is still typing a space into does not stop
	// matching, and so a box holding nothing but spaces is the whole manual
	// rather than a search for a space.
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.MonsterNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	monsters, err := a.monsterList(ctx, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to search monsters", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterCardsFragment(pages.MonsterListData{Monsters: monsters, Query: term}))
}

// monsterList is the manual in either of its two states, so the page and the
// search fragment build the same cards from the same function -- and a card
// that appeared only in one of them cannot exist.
//
// The term is escaped by journalSearchPattern rather than by a second copy of
// it. What LIKE reads as a pattern is a fact about MySQL and not about journals:
// an unescaped `%` matches the whole manual here exactly as it matches the whole
// journal there.
func (a *App) monsterList(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MonsterSummary, error) {
	var rows []queries.Monster
	var err error

	if term == "" {
		rows, err = a.Queries.ListMonsters(ctx, ownerID)
	} else {
		rows, err = a.Queries.SearchMonsters(ctx, queries.SearchMonstersParams{
			OwnerID: ownerID,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}

	monsters := make([]pages.MonsterSummary, 0, len(rows))
	for _, row := range rows {
		monsters = append(monsters, monsterSummary(row))
	}

	return monsters, nil
}

// monsterSummary is one row as its card reads it. The three chips are the
// readings a GM picks a monster by; the rest of the stat block is behind the
// View button.
func monsterSummary(monster queries.Monster) pages.MonsterSummary {
	image := ""
	if monster.AssetID != nil {
		image = monster.AssetID.String()
	}

	return pages.MonsterSummary{
		ID:       monster.ID.String(),
		Name:     monster.Name,
		Subtitle: monsterSubtitle(monster),
		ImageID:  image,
		CR:       pages.ChallengeRatingLabel(pages.NormalizeChallengeRating(monster.CR)),
		AC:       strconv.FormatUint(uint64(monster.AC), 10),
		HP:       strconv.FormatUint(uint64(monster.HP), 10),
	}
}

// MonsterPage is the editor: the stat block down the left, the panels down the
// right. Two queries, and they are the same two every save reads back -- the row
// and its action rows -- because the block is built from both.
func (a *App) MonsterPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monster, monsterID, ok := a.loadMonster(w, r)
	if !ok {
		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load monster actions", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.EditMonster(monsterToEditPageData(monsterID.String(), monster, actions)))
}

// loadMonster is the ownership gate the editor goes through, and the only one: a
// page that asked the question a second way would answer a miss differently
// sooner or later. Every failure is a redirect rather than an alert, because
// this answers a page request and nothing is open yet to show an alert in. A
// monster that is not this user's and one that never existed are the same miss,
// because the query is scoped to the owner.
//
// The parsed id comes back with the row, because the caller goes on to query the
// action rows with it and would otherwise parse the path twice.
func (a *App) loadMonster(w http.ResponseWriter, r *http.Request) (queries.Monster, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/monsters")
		return queries.Monster{}, ulid.ULID{}, false
	}

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/monsters")
		return queries.Monster{}, ulid.ULID{}, false
	}
	if err != nil {
		slog.Error("Failed to load monster", "error", err)
		redirectToError(w, r)
		return queries.Monster{}, ulid.ULID{}, false
	}

	return monster, monsterID, true
}

// MonsterStatBlockFragment is the block on its own, for the View dialog on the
// manual -- and it is the URL a pawn will open when the VTT exists, which is why
// it takes the monster in the query string rather than in a path. This is not
// the monster's URL; it is a representation of it that something else opens.
//
// IT IS OWNER-SCOPED AND STAYS THAT WAY. A room's players will need this block
// too, and what they will need is a second route scoped to room membership --
// not a relaxation of this one.
//
// An id that will not parse is a 404 with an empty body: it came off the page's
// own markup, so a request carrying a broken one is not a reader who has lost a
// monster and has nothing to be told. A monster that is gone is the opposite --
// somebody clicked View on a card for a row deleted in another tab -- so that
// one gets the alert.
func (a *App) MonsterStatBlockFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.URL.Query().Get("monster"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to load monster", "error", err)
		htmx.ServerError(w)
		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load monster actions", "error", err)
		htmx.ServerError(w)
		return
	}

	derived := monsterDerived(monster, actions)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterStatBlockFragment(monsterStatBlock(monster, actions, derived)))
}

// monsterToEditPageData is every control on the editor, filled in. Each field is
// the string that goes into a value attribute, converted here so the markup does
// none of it -- which is also what lets the empty struct render an empty editor,
// as the concurrency test does to every page.
func monsterToEditPageData(id string, monster queries.Monster, actions []queries.MonsterAction) pages.EditMonsterPageData {
	derived := monsterDerived(monster, actions)

	return pages.EditMonsterPageData{
		MonsterID: id,
		Header:    monsterHeader(monster, derived),
		StatBlock: monsterStatBlock(monster, actions, derived),

		Name: monster.Name,
		// The three pickers are normalised on the way out as well as on the way
		// in. A column written before a list changed would otherwise render a
		// select with nothing chosen, and the next save would store whatever the
		// browser picked for it.
		Size:      pages.NormalizeSize(monster.Size),
		Type:      pages.NormalizeCreatureType(monster.Type),
		Tags:      monster.Tags,
		Alignment: pages.NormalizeAlignment(monster.Alignment),

		Str: strconv.FormatUint(uint64(monster.Str), 10),
		Dex: strconv.FormatUint(uint64(monster.Dex), 10),
		Con: strconv.FormatUint(uint64(monster.Con), 10),
		Int: strconv.FormatUint(uint64(monster.Int), 10),
		Wis: strconv.FormatUint(uint64(monster.Wis), 10),
		Cha: strconv.FormatUint(uint64(monster.Cha), 10),

		AC:                        strconv.FormatUint(uint64(monster.AC), 10),
		HP:                        strconv.FormatUint(uint64(monster.HP), 10),
		HitDice:                   monster.HitDice,
		Speed:                     monster.Speed,
		InitiativeBonus:           strconv.FormatInt(int64(monster.InitiativeBonus), 10),
		CR:                        pages.NormalizeChallengeRating(monster.CR),
		LegendaryActionUses:       strconv.FormatUint(uint64(monster.LegendaryActionUses), 10),
		LegendaryActionUsesInLair: strconv.FormatUint(uint64(monster.LegendaryActionUsesInLair), 10),

		// The six defense boxes and the two description words are NOT NULL with
		// an empty default and the builders trim what they store, so they pass
		// straight through -- there is nothing for a fallback to do.
		Vulnerabilities: monster.Vulnerabilities,
		Resistances:     monster.Resistances,
		Immunities:      monster.Immunities,
		Gear:            monster.Gear,
		Senses:          monster.Senses,
		Languages:       monster.Languages,

		Habitat:     monster.Habitat,
		Treasure:    monster.Treasure,
		Description: monster.Description,

		Derived: derived,
		Actions: monsterActionRows(actions),
	}
}

// monsterActionRows groups the rows by the section they belong to, which is how
// the editor draws them: it ranges over the section list and asks this map for
// each one, so a section with no rows renders its heading and its add button and
// nothing else.
func monsterActionRows(actions []queries.MonsterAction) map[string][]pages.MonsterAction {
	rows := map[string][]pages.MonsterAction{}
	for _, action := range actions {
		kind := string(action.Kind)
		rows[kind] = append(rows[kind], monsterActionPageRow(action))
	}

	return rows
}

// NewMonsterFragment serves the content of the new-monster dialog: a heading,
// one field and two buttons. It reaches no database and carries nothing from the
// session, because the form is the same for every user -- but it stays behind
// auth.Fragment all the same, since an unauthenticated route here would be
// surface for no reason.
func (a *App) NewMonsterFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewMonsterFragment())
}

// NewMonsterForm creates a monster from a name, gives it the picture the dialog
// carried if there was one, and sends the browser to the editor. Everything else
// in the stat block is answered by the schema and filled in afterwards by a page
// that saves as you go.
//
// THE PICTURE IS CHECKED BEFORE THE MONSTER EXISTS. Decoding is the only part of
// an upload a person can be at fault for, and doing it first is what lets a file
// that will not open come back as a sentence over a dialog that still holds the
// name they typed. Everything after the create is a server fault, and none of it
// is worth throwing the monster away over -- the editor carries the same upload
// control, so a picture that did not land can be added there.
//
// The reply on success is a redirect with no body, so nothing lands back in the
// dialog -- the navigation takes it away. The toast still arrives, on the page
// after this one: toast.js parks a message in sessionStorage when the same
// response also carries HX-Redirect, because a message shown a moment before a
// navigation is never read.
func (a *App) NewMonsterForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	// The dialog posts multipart because it may carry a file. A form that sent
	// its fields the ordinary way is still a create with no picture, and
	// parsing has already read the name out of it by the time it says so.
	if problem := parseUploadForm(w, r, imageLimits); problem != nil && problem != errNotMultipart {
		rejectNewMonster(w, r, problem.Message)
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectNewMonster(w, r, "Name is required.")
		return
	// Characters, not bytes: the column is varchar(128) and MySQL counts
	// characters there, so len() would refuse a name of ninety accented
	// letters that the database would have taken.
	case len([]rune(name)) > pages.MonsterNameLimit:
		rejectNewMonster(w, r, "Name must be 128 characters or fewer.")
		return
	}

	picture, filename, ok := a.newMonsterPicture(w, r)
	if !ok {
		return
	}

	id := ulid.Make()
	err := a.Queries.CreateMonsterFromName(ctx, queries.CreateMonsterFromNameParams{
		ID:      id,
		OwnerID: sess.UserID,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to create monster", "error", err)
		htmx.ServerError(w)
		return
	}

	created := name + " has been created."
	if picture != nil {
		if _, err := a.attachMonsterImage(ctx, sess.UserID, id, picture, filename); err != nil {
			slog.Error("Failed to attach the new monster's picture", "error", err, "monsterID", id.String())
			created = name + " has been created, but the picture could not be saved. Add it again from the editor."
		}
	}

	htmx.Toast(w, created)
	htmx.Redirect(w, "/monsters/"+id.String()+"/edit")
}

// newMonsterPicture is the dialog's optional file, decoded and re-encoded to
// what a monster's picture is stored as. It answers (nil, "", true) when the
// field was left alone, which is the ordinary case -- the whole dialog is one
// question and this is beside it.
//
// A bad file is a 422 into the dialog's error block rather than an alert, for
// the reason every other failure on this form is: the modal stays open on the
// thing that needs fixing, with the name still in the field. An alert would open
// a second dialog over the first to say the same sentence.
func (a *App) newMonsterPicture(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	file, filename, problem := openOptionalImageUpload(r, "image", imageLimits)
	if problem != nil {
		rejectNewMonster(w, r, problem.Message)
		return nil, "", false
	}
	if file == nil {
		return nil, "", true
	}
	defer file.Close()

	src, err := imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		slog.Warn("Failed to decode a new monster's picture", "error", err)
		rejectNewMonster(w, r, errUnsupportedImage.Message)
		return nil, "", false
	}

	encoded, err := images.EncodeWebP(images.Square(src, monsterImageSize))
	if err != nil {
		slog.Error("Failed to encode a new monster's picture as webp", "error", err)
		htmx.ServerError(w)
		return nil, "", false
	}

	return encoded, filename, true
}

// rejectNewMonster answers with the dialog's error block under a 422, which is
// the one code the form has an hx-status route for -- every other 4xx is in the
// noSwap list in base.templ and would leave the dialog showing nothing new. The
// form is left alone, so the name the user typed is still in the field when the
// message appears above it.
func rejectNewMonster(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewMonsterPanel, []string{message}))
}

// DeleteMonster empties every table that holds a row for this monster, deletes
// the object its picture points at, and only then removes the monster itself.
// It is DeleteCharacter's order and it is that order for the same reasons.
//
// NOTHING CASCADES IN THIS SCHEMA -- there are no foreign keys -- so every table
// is named by hand, and a row this handler forgets is unreachable the moment the
// monster is gone: no page can open it and no later delete will find it.
//
// THE OBJECT GOES BEFORE THE ROW THAT DESCRIBES IT. The assets row is the record
// that an object may exist, so R2 goes first; the other way round leaves a
// bucket filling with keys nothing remembers, and the sweeper cannot find them
// either, because it works from those same rows.
//
// THE MONSTER ROW GOES LAST, which is what makes every failure above it
// recoverable. While it is there the manual still lists the monster and deleting
// it again re-runs the whole purge: every statement is scoped by the monster and
// the owner, so repeating one finds nothing and succeeds, and a key already gone
// from R2 deletes again without complaint.
//
// Past that row there is nothing left to find, which is why the asset row -- the
// one step after it -- is logged rather than reported: a retry could not reach
// it, and the user's request has been honoured.
func (a *App) DeleteMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
		return
	}

	monster, err := a.Queries.GetMonsterAsset(ctx, queries.GetMonsterAssetParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to query monster asset", "error", err)
		htmx.ServerError(w)
		return
	}

	if monster.FilePath.Valid {
		if err := a.Storage.Delete(ctx, monster.FilePath.String); err != nil {
			slog.Error("Failed to delete monster image object", "error", err)
			htmx.ServerError(w)
			return
		}
	}

	if err := a.deleteMonsterRows(ctx, monsterID, sess.UserID); err != nil {
		slog.Error("Failed to delete monster rows", "error", err)
		htmx.ServerError(w)
		return
	}

	err = a.Queries.DeleteMonster(ctx, queries.DeleteMonsterParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete monster", "error", err)
		htmx.ServerError(w)
		return
	}

	if monster.AssetID != nil {
		err := a.Queries.DeleteAsset(ctx, queries.DeleteAssetParams{
			ID:      *monster.AssetID,
			OwnerID: sess.UserID,
		})
		if err != nil {
			slog.Error("Failed to delete monster image asset row; leaving it behind", "error", err, "assetID", monster.AssetID.String())
		}
	}

	htmx.Toast(w, monster.Name+" has been deleted.")
}

// deleteMonsterRows empties every table that carries this monster's rows, and it
// is separate from the handler because the list is the point: one statement per
// table, in one place, so a table added to the schema has an obvious hole to
// fill. TestDeletingAMonsterEmptiesEveryTableThatHoldsItsRows reads db/schema.sql
// and fails when one is missing.
//
// THE SHAPE IS FOR THE TABLES THAT ARE NOT HERE YET. A monster's rows are its
// seven sections and the link it may have been shared by -- and the VTT work
// will add tables that hang off a monster the way spells and inventory hang off
// a character. Two statements inline in the handler would be the thing that gets
// forgotten then.
//
// THE SHARE ROW IS FOUND BY resource_id AND NOT BY A monster_id COLUMN, which is
// the one table here the schema scan cannot see -- shares names what it points
// at by type and id, so a monster's link is a row whose resource_type says
// monster. TestDeletingAMonsterEmptiesEveryTableThatHoldsItsRows names it
// outright for that reason. Leaving it behind would be a live link to a monster
// nobody can open: the reader would find the share, fail to find the monster,
// and be told the link is dead -- true, but the row would sit there until the
// account went.
//
// The failure is returned wrapped, so the log names the table rather than only
// the driver error.
func (a *App) deleteMonsterRows(ctx context.Context, monsterID, ownerID ulid.ULID) error {
	if err := a.Queries.DeleteMonsterActions(ctx, queries.DeleteMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	}); err != nil {
		return fmt.Errorf("monster actions: %w", err)
	}

	// A monster has one link at most, so revoking it and purging it are the
	// same statement -- unlike a character, which has one of its own plus one
	// per journal entry and needs a wider delete beside the narrow one.
	if _, err := a.Queries.DeleteMonsterShare(ctx, queries.DeleteMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	}); err != nil {
		return fmt.Errorf("monster share: %w", err)
	}

	return nil
}
