# Tabletopper server review

Date: 2026-10-12. Branch: `beta` at `f62f987`.

Scope: every non-generated Go file under `server/` (main, routes, all 26 internal
packages, the templ `.go` helpers), the SQL under `server/sql` and `db/schema.sql`,
the layouts and the room page template, and the client modules that carry state
(`socket.ts`, `main.ts`, `window.ts`, `music.ts`, `store.ts`, `effects.ts`, the modal
and toast scripts). The rest of the templ markup and the renderer were skimmed for
raw HTML, inline script and `innerHTML`, not read line by line.

Baseline: `go build`, `go vet`, `gofmt -l` and `go test ./...` are all clean.

## Verdict

The foundations are sound and most of the hard problems are already solved well:
the room actor with per-role sequence numbers, derived change frames and resync;
the lease-based map tiler with generations; session hashing, bcrypt slot limiting,
share tokens, goldmark with raw HTML and `javascript:` links rejected, the
`/fragment/` discipline, and the modal and window contracts. There is no XSS or
auth bypass I could find. The client never touches `innerHTML`.

The findings below are therefore mostly about three things:

1. A handful of real bugs, two of which bite users today (a cached 404 that pins
   missing tiles for a year, and stale CSS and JS for an hour after a deploy).
2. Places where the data layer leaks into handlers and views, which is what will
   make the next feature slower to add than the last one.
3. Idioms: the code is already good Go. The notes are about consistency, not
   correctness.

Findings are numbered so they can be referred to. Severity is my estimate of user
impact, not effort.

## 1. Bugs

### B1. A failed image fetch is cached as a 404 for a year — high

`server/internal/controllers/assets.go:170` `streamImage` writes `404` when R2
returns an error, but the callers have already set the cache policy:

- `map-tiles.go:106` `Cache-Control: private, max-age=31536000, immutable`
- `journal-images.go:129` same
- `share.go:193` same

A 404 carrying an explicit `max-age` is cacheable, and browsers do store it. One
transient R2 error, timeout or throttle on a tile request leaves that tile missing
in that browser until the cache is cleared. The ETag does not help because the
browser never revalidates an immutable response.

Fix: set the cache header only on the success path. Pass the policy into
`streamImage` and set it after `Storage.Get` returns, or `Del("Cache-Control")`
before writing the 404. The same applies to the `304` branch, which is fine, and
the `context.Canceled` branch, which should not write a cacheable status either.

### B2. Static assets are cached for an hour but not versioned — high

`server/routes.go:257` serves `/css/`, `/js/`, `/static/` and `/images/` with
`Cache-Control: public, max-age=3600`. Only the room bundle is versioned
(`RoomPageData.Bundle` appends `?v=`). Every other asset in
`server/templ/layouts/base.templ` (`app.css`, `htmx.min.js`, the modal, toast and
theme scripts, `sound-preview.js`, `journal-editor.js`) is referenced bare.

After a deploy, a user with a warm cache gets new HTML with old CSS and old modal
or socket JavaScript for up to an hour. Because the room page reloads itself on a
version mismatch, a client whose cached `room.js` is current but whose `events.js`
or `content-modal.js` is stale can land in a state no test covers.

Fix: a `layouts.Asset(path string) string` helper that appends `?v=` plus
`hub.Version()` (already computed from VCS metadata) and use it for every asset in
the base and share layouts and in `clerk.templ`. Then the hour-long `max-age` is
safe, and could be a year. Until then, `no-cache` on the unversioned paths is the
correct policy, because `http.FileServer` already answers `If-Modified-Since`.

### B3. Hub errors on a closed room surface as 500s, and the closed room still offers the commands — medium

`room-socket.go:147` `rejectCommand` treats anything that is not a `*room.Error` as
a server fault: it logs at `Error` and answers 500. `hub.ErrNoRoom` (the room is
closed, so `GetRoomSnapshot` returns no row) and the hub's `errGone` both arrive
this way.

This is reachable. `templ/pages/room.go:332` `tabletopMenu` and `:423`
`initiativeMenu` are rendered for the GM whether or not the room is closed, so
"Spawn pawns", "Clear tabletop", "Sync tracker" and the rest are one click from a
500 and an error log on every closed room.

Fix both sides: hide the tabletop, fog and initiative menus when `Closed`, and map
`hub.ErrNoRoom` to `htmx.NotFound(w, "room")` and context errors to a 503 in
`rejectCommand`. Export the hub's gone error or wrap it so callers can
`errors.Is` it.

### B4. Deletes remove the objects before the rows, on the request context — medium

`characters.go:53` and `:59`, `monsters.go:360`, `assets.go` `DeleteMap` and
`library-assets.go:290` all delete from R2 first and only then open the transaction
that deletes the rows. If the transaction fails, or the client disconnects between
the two, the user sees a 500 and a character whose portrait and journal images are
gone. A retry recovers because deleting a missing key succeeds, but nothing tells
the user that.

The reverse order is strictly better: delete the rows in one transaction (the
entity is gone from every list immediately), then delete the objects on a
`storage.CleanupContext` so a disconnect cannot interrupt it. Orphaned objects
cost cents; dangling rows cost support time. See S3 below for turning this into
one mechanism instead of five.

### B5. Every committed command deep-clones the whole room state five times — medium (performance)

`hub/actor.go:271` clones the state before `Apply`. `broadcast` then runs
`reflect.DeepEqual` over the whole state, and `room.Derive` (`room/derive.go:10`)
calls `Project` on both states for both roles, which is four more full clones,
followed by per-section `DeepEqual` and map-building diffs.

For pawn moves this is fine. For free-hand drawing it is not: every
`stroke.extend` chunk (a committed command, up to 15 a second per drawer) clones
every stroke in the room (up to 200,000 coordinates), twice per role, and compares
them. Fog shapes and tiles ride along. With several players drawing on a full map
this is the one place the server will visibly fall behind.

Two changes remove most of it with no behaviour change:

- `Project(RoleGM)` is `Clone()`. `Derive` only reads, so for the GM role pass
  `before` and `a.state` through directly instead of cloning them. That is two of
  the five clones.
- The GM diff being empty is exactly "nothing changed", so `dirty` and `changes`
  can come from `len(gm) > 0` and the whole-state `DeepEqual` in `broadcast` goes.

A further step, if it is ever needed, is for commands to say which sections they
touch so `Derive` compares only those. The `Coalescer` interface is the model for
that kind of opt-in.

### B6. The tiler can need around 800 MB for one map — medium (operational)

`assets.go:35` allows 150 megapixel maps. `tiler.Build` decodes the whole image
(600 MB of RGBA at that size), then `halve` allocates the next level while the
previous one is still live, and sixteen encoders hold tile buffers on top. The two
decode slots that protect the upload handlers do not cover the tiler. Nothing in
`compose.yaml` or the Dockerfile sets a memory limit or `GOMEMLIMIT`.

A single large upload on a small VPS will take the whole server down, not just the
tiler. Either lower the cap substantially (40 MP already covers a 6k by 6k battle
map), or decode and downscale in one pass before building the pyramid, and set
`GOMEMLIMIT` so the runtime at least tries.

### B7. A socket whose session expired reconnects forever — low

`hub/conn.go:149`: when the membership recheck fails the client is stopped with
`closePolicy` and an empty reason. `socket.ts` treats an empty reason as a plain
close, reconnects, gets a 404 from the handshake, and repeats every 15 seconds for
as long as the tab is open. Name the reason (`reasonExpired`) and add it to
`ENDED_REASONS` so the client sends the user to sign in.

### B8. A tiling job interrupted by a restart waits fifteen minutes — low

On SIGTERM the tiler's context is cancelled, `abandon` (`tiling/job.go:172`)
returns early, and the asset stays `working` with its lease. The reclaim pass runs
at boot, but only reclaims leases older than `leaseWindow`, so a map that was
being tiled during a deploy shows "working" for up to fifteen minutes afterwards.
On cancellation, requeue the job with a short background context instead.

### B9. Sheet write-through and sheet sync can cross — low

`hub/effects.go` writes pawn vitals back to the character row asynchronously
through `sheetWriter`. Saving any character panel (`finishCharacterPanel`) sends a
`CharacterSync` that overwrites the pawn's HP, max HP and AC from the row. If a
player saves their Personality panel a moment after the GM changes their pawn's
HP, and the write-through has not landed, the pawn's HP reverts to the sheet's
older value. The window is tens of milliseconds, so this is a note rather than a
fix request. If it ever matters, `CharacterSync` should only push vitals when
the panel that was saved is the vitals panel.

### B10. Small ones

- `journals.sql` `CountJournalImages` counts detached images, so an entry whose
  images were deleted and are waiting for the 24-hour sweep still hits the
  40-image limit.
- `UpdateAssetName` (rename of maps and library assets) and `SetUserAvatar` are
  `:exec` with no rows-affected check, so renaming a deleted asset toasts success.
- `attachMonsterImage` and `storeMap` write the raw browser filename into `name`
  while every other upload path goes through `assetName`. Consistent today only
  because browsers cap filenames at 255 bytes.
- `SaveScene` and `DuplicateScene` count then insert, so the 200-scene cap is
  advisory under concurrency.
- `routes.go:265` `notFound` logs every 404 at `Warn`. Crawlers will fill the log.
  `Info` or nothing.
- `room-page.go` `roomMember` writes to the session (`LeaveRoom`) from a read
  path when a player's room has closed. It works, but a function called
  `roomMember` that mutates the session surprises the next reader.

## 2. Security notes

Nothing here needs urgent action. These are decisions to make consciously.

- **Images and tiles are capability URLs.** `GET /assets/images/{id}` and
  `GET /assets/maps/{id}/tiles/...` check for a session but not ownership
  (`assets.sql` `GetImage`, `GetMapPyramid`). This is deliberate: players must see
  the GM's tokens and maps. It also means any signed-in user who learns a ULID can
  fetch another user's character portrait, profile picture or monster art. ULIDs
  have 80 bits of entropy so they are not guessable, but they do appear in page
  source, logs and share pages. Worth one line in the README or CLAUDE.md so it is
  never "fixed" by accident in a way that breaks rooms, and worth not including
  `profile` and `character` types in `GetImage` unless something needs them.
- **Share links use `r.Host`.** `share-form.go` `shareLink` builds the link from
  the Host header. Behind a proxy that forwards Host this is right; it is also the
  one place a spoofed Host header shows up in UI. Prefer a configured public base
  URL.
- **No CSP or HSTS at the app level.** `middleware/headers.go` sets nosniff,
  frame-deny and referrer policy. The share pages set `img-src 'self'`. If the
  proxy does not add HSTS, add it here. A full CSP is harder because of Clerk's
  scripts, but a report-only one would tell you what it would break.
- **Good:** `http.NewCrossOriginProtection` plus `SameSite=Lax` cookies, bcrypt
  bounded by `GOMAXPROCS` slots with a context, share tokens validated before the
  query, journal markdown images rewritten to the share's own image route and
  anything else stripped, music type sniffed from the object header on confirm,
  `DisallowUnknownFields` on wire commands, and every `hx-vals` built from ULIDs
  and integers only.

## 3. High-value structural changes

These are the changes I would make before the next large feature. Each one
removes a category of future bugs rather than a single bug.

### S1. Move the game rules out of the view package

`controllers/rules.go` holds XP thresholds, proficiency and the stat block
assembly. `templ/pages/monster-options.go`, `character-options.go`, `bonuses.go`
and `limits.go` hold sizes, alignments, creature types, challenge ratings, skills,
saving throws and the normalisers. Controllers import `pages` to get at them.

That is the dependency pointing the wrong way: the view package owns the domain
vocabulary, and the handlers reach into the view to validate input. A 2024-rules
change, a new creature size or a homebrew skill touches `pages`, `controllers`,
`room` (`CreatureSize`, `Size.Values`) and the export package.

Make an `internal/dnd` package (or `rules`) that owns sizes, alignments, CRs,
skills, proficiency math, XP tables and the normalisers. `controllers`, `pages`,
`room` and `export` all import it; nothing imports `pages` except `controllers`
and `export`. `room.Size` and `pages.SizeLabel` would then agree by construction.

### S2. Put the queries behind feature-sized interfaces

`App` carries `*queries.Queries` and every handler calls it directly, so the
handler tests fake a SQL driver: `onerow_db_test.go`, `room_db_test.go` and
`middleware/session_db_test.go` each implement `driver.Conn`, `driver.Stmt` and
`driver.Rows` by hand and answer by matching substrings of the SQL. Those fakes
break every time a query gains a column, and they test the fake as much as the
handler.

The repo already has the right pattern in two places: `tiling.rows` and
`hub.Store` are small interfaces satisfied by `*queries.Queries`, and their tests
pass plain structs. Do the same per feature: a `characterStore`, `shareStore`,
`assetStore` interface in `controllers` listing only the methods that feature
calls. Handlers take the interface; tests pass a map-backed fake. This is also
what makes the 29 `if a.Hub == nil` guards go away: a no-op `Hub` implementation
for tests instead of a nil check in every handler.

### S3. One asset-deletion mechanism instead of five

Journal images already have the right design: deleting detaches the row
(`detached_at`) and `sweep.JournalImages` removes the objects later. Character
portraits, monster art, maps and library assets each hand-roll the opposite
order (B4) with their own error handling.

Generalise the sweep: every delete marks the asset row detached inside the
entity's transaction, and one sweeper deletes objects for any detached asset of
any type and then the row. Deletes become a single transaction, R2 outages stop
turning into 500s on delete, and `discardAsset`'s cleanup context dance in the
upload paths can go too, because an upload that fails after the insert just
marks the row detached.

### S4. Collapse the three share dialogs and the fragment boilerplate

`character-share.go`, `journal-share.go` and `monster-share.go` are the same 150
lines three times with different query names. `library-assets.go` shows how to do
it once: `libraryKind` is a small struct of functions and the handlers are
one-liners. A `shareKind{Heading, Blurb, Action, Get, Insert, Delete}` would do
the same here, and the fourth shareable resource would be twelve lines.

Likewise `w.Header().Set("Content-Type", "text/html; charset=utf-8")` followed by
`render` appears in roughly fifty fragment handlers, and the
`ulid.Parse(r.PathValue("id"))` then `htmx.NotFound` pair in forty more. A
`renderFragment(w, r, c)` and a `pathID(w, r, name, what) (ulid.ULID, bool)` would
remove several hundred lines and make the fragment contract one function.

### S5. One sweep loop

`sweep/journal_images.go`, `sweep/shares.go`, `sweep/music_uploads.go`,
`sweep/accounts.go`, `session/cleanup.go` and `tiling/tiling.go` each contain the
same ticker goroutine with the same `ctx.Done` branch and the same log line. A
`sweep.Every(ctx, name string, interval time.Duration, fn func(context.Context))`
makes each of them three lines and gives you one place to add jitter, a startup
delay, or a metric.

### S6. Configure logging

There is no `slog.SetDefault` anywhere, so the server logs with the default text
handler at `Info`, and every `slog.Debug` call in the codebase is dead. Pick the
handler and level from `ENV` in `main.go`: JSON in production, text in
development, `Debug` when developing. Five lines.

## 4. Idiomatic Go

Things done well, so they are kept: `errors.Is` and `errors.As` throughout,
`sql.Result` rows-affected checks on updates, `context.WithoutCancel` for work
that must outlive a request, generics used where they pay for themselves
(`clamped`, `countValue`, `hub.view[T]`, `room.diff[T]`), the `Values()` and
`Valid()` convention on string enums with a generator that keeps the TypeScript
union in step, and the command pattern in `room` with `Authorize`, `Resolve` and
`Apply` kept separate.

Nits, in the order I would fix them:

- `controllers/assets.go` `uploadProblem` is an error that does not implement
  `error`, and `errNotMultipart` is compared by pointer identity in
  `NewMonsterForm` and `CreateSpawnMonster`. Give it an `Error()` method and use
  `errors.Is`.
- `assets.go` `requeueMap(w, ctx, ...)` takes the context second. `share.go`
  `sharePortrait(w, r, a, ...)` takes the receiver as an argument. Both read as
  methods that lost their receiver; make them methods.
- `room/resolve.go` and `hub/hub.go` each define an unexported `gone(err)`.
  Export one (`room.IsGone`) and delete the other.
- `room/command.go` `cloneSlice` reimplements `slices.Clone`, with the one
  difference that it returns an empty non-nil slice for nil input. `Normalize`
  already handles nil, so `slices.Clone` would do.
- `share-monster.go` `copyMonsterActions` builds its own `ulid.Monotonic`
  entropy; `ulid.Make()` is already monotonic and safe for concurrent use.
- `hub/hub.go` `Dispatch` retries with `for range 2`. The literal is the number
  of times a command is retried when the room unloads under it; give it a name
  such as `reloadRetries` so the next reader does not have to work that out.
- `room-page.go` and `room-table.go` both define `errNotAMember`-style
  membership checks with slightly different return shapes (`roomMember`,
  `loadRoomMember`, `gmTable`, `pawnActor`, `sceneRoom`, `dispatchRoom`,
  `spawnRoom`). They are all right, but there are seven of them. Two would do:
  one that returns the row and role, one that additionally requires the GM.
- `controllers/characters.go` `parseFeatures` returns `[]pages.Feature{}` in
  three places to avoid a nil slice. If the template handles nil, drop it; if it
  does not, the template should.
- `room/code.go` ignores the error from `rand.Read`. Fine since Go 1.24 (it
  cannot fail), but a reader has to know that; `rand.Text` or a one-line
  justification in the commit message would spare them.

## 5. Client notes

- `socket.ts`, `window.ts` and `store.ts` are tidy and match the server's
  contracts. The resync-on-gap logic and the per-role sequence numbers line up.
- `public/js` is the one place in the repository with comments (16 files, around
  600 comment lines). The project rule says none, anywhere. Either the rule has an
  exception for hand-maintained browser scripts, in which case CLAUDE.md should
  say so, or these should be stripped like everything else.
- `music.ts` `target()` divides by `audio.duration` for looping; when metadata
  has not loaded yet `span()` is 0 and the modulo is skipped, which is correct,
  but `seek` then runs on a `src` that has not loaded and is silently swallowed.
  It resyncs on `loadedmetadata`, so this is fine; noting it so nobody "fixes"
  the empty catch.
- `window.ts` keys saved window paths by room but geometry by id only, as
  CLAUDE.md says. The `RETIRED_WINDOWS` list is the right place for migrations of
  that storage and should grow as window ids change.

## 6. Tests and tooling

- `go test ./...` passes; `go vet` and `gofmt` are clean; `make check` wires in
  the TypeScript tests. Good.
- The three hand-written `database/sql/driver` fakes are the brittle part (S2).
- `routes_test.go` and the `uievents` test that greps the JavaScript for event
  names are exactly the kind of test this codebase benefits from: they enforce a
  contract that nothing else would catch.
- There is no test for `streamImage` on a storage error, which is why B1
  survived. A table test over "cache header present after a 404" would have
  caught it and will keep it caught.

## 7. Suggested order

1. B1 and B2, together, in one change: both are header policy and both are
   user-visible today.
2. B3: hide the menus on a closed room and map the hub errors.
3. B6: lower the map pixel cap and set `GOMEMLIMIT`, before anyone uploads a
   scan.
4. S3 with B4: the deletion mechanism, since it touches every delete handler and
   is easiest to do once.
5. B5: the two cheap clone removals in the actor.
6. S1, then S2, as the next feature's first step rather than as a standalone
   refactor, so each lands with a consumer.
7. S4, S5, S6 and the section 4 nits as they are passed by.
