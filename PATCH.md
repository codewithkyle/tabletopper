# Tabletopper patch plan

Source: `REVIEW.md` dated 2026-10-12, branch `beta` at `f62f987`. Nothing from the
review has been applied. Line numbers below are as of that commit; they drift as
phases land, so verify each anchor before editing.

The work is split into eight phases. Each phase ends with a stop point: the
automated checks to run and a manual procedure with what to check and what to
expect. Do not start the next phase until the stop point passes. Phases 1 to 5
are the bugs and the operational fixes; 6 to 8 are the structural changes, each
of which is safe to defer or to fold into the next feature.

| Finding | Phase | Finding | Phase |
| --- | --- | --- | --- |
| B1 cached 404 images | 1 | S1 `internal/dnd` | 6 |
| B2 unversioned assets | 1 | S2 store interfaces | 8 |
| B3 closed-room 500s | 2 | S3 one deletion mechanism | 4 |
| B4 object-before-row deletes | 4 | S4 share kinds, fragment helpers | 7 |
| B5 five clones per command | 5 | S5 `sweep.Every` | 7 |
| B6 tiler memory | 3 | S6 logging | 3 |
| B7 expired socket loop | 3 | Security notes | 3 |
| B8 tiling waits 15 min | 3 | Go nits | 2, 5, 6 |
| B9 sheet sync race | not planned | Client notes | 7 |
| B10 small ones | 1, 4 | | |

## Ground rules for every phase

- The rules in `CLAUDE.md` hold: no code comments in any file, no git commit or
  push, every partial-HTML GET under `/fragment/`, the three modals and nothing
  else, the DaisyUI skill loaded before any templ markup is written.
- The one tolerated exception to the comment rule is the pair of dbmate markers
  that every file in `db/migrations` already carries, because dbmate will not
  run a migration without them. Copy the neighbouring file's shape exactly.
- Regenerate after edits: `make templ` after a `.templ` change, `make sqlc` after
  `server/sql/*.sql`, `make protocol` after a `room` enum change, `make js` after
  anything under `server/js`, `make css` after a class change in templ.
- `make check` (gofmt, templ fmt, vet, `go test -race`, TypeScript tests) is the
  automated gate at every stop point. It must be green before the manual steps.
- Tests that exist for a handler being changed are updated in the same change,
  never deleted and re-added later.
- Each phase is one working-tree change the user can review and commit as a
  unit. Do not mix phases.

## Decisions to make before starting

These change what gets built. The phases below assume the recommended answer.

1. **Deletion mechanism (Phase 4).** The review proposed reusing
   `assets.detached_at` for every asset type. Checking the read side changes the
   recommendation: a detached map, token, avatar or monster image would still be
   returned by `GetImage`, `GetMapPyramid`, the library lists, the map list and
   the room's hydrate-time `Prune` until the sweep ran, so every one of those
   queries would need a `detached_at IS NULL` guard. Recommended instead: a small
   `object_purge` table that each delete transaction inserts the object keys
   into, drained by one sweeper. Rows are deleted exactly as today; only the R2
   half moves off the request. Journal images keep `detached_at` because they
   need re-attach semantics.
2. **Map pixel cap (Phase 3).** Recommended 60 megapixels (an 8k by 7.5k map).
   The review suggested 40; that would equal the decoded-image cap and make the
   test `TestTheMapCapTakesWhatTheDecodedCapCannot` meaningless. 60 keeps a
   clear gap and brings the tiler's worst case under about 350 MB.
3. **Comments in `server/public/js` (Phase 7).** Either strip them, which the
   project rule requires, or add an explicit exception to `CLAUDE.md`. The plan
   assumes strip.
4. **HSTS (Phase 3).** Only add it at the app level if the reverse proxy does
   not already send it. Check the proxy's response headers first; a duplicate
   header is worse than none.

## Phase 1. Cache policy

**Status: done 2026-10-12.** `make check` green and stop point passed. The plan
below is kept as written; what was built differs in the ways listed under
"Changes from this plan", and those take precedence.

User-visible today. All three items are header policy; none change markup
beyond the asset URLs.

### Changes from this plan

- **Assets are versioned by path, not by query.** `layouts.Asset(path)` returns
  `/v/<version>` plus the path, and the routes are
  `GET /v/{version}/css/`, `/js/`, `/static/` and `/images/`. The `?v=` design
  would have run `toast.js` twice: Base loads it directly, while `modal.js` and
  `public/js/room.js` import `./toast.js`, which resolves to the bare
  `/js/toast.js`. Browsers key modules by full URL, so that is two instances,
  two `htmx:after:request` listeners and every toast shown twice. With the
  version in the path a relative import inherits it, so the whole module graph
  shares one URL space. The bare imports (`events.js`, `modal.js`, `toast.js`)
  are therefore immutable too, rather than `no-cache` as planned.
  `TestARelativeImportResolvesToTheURLTheLayoutLoads` in
  `templ/layouts/layouts_test.go` fails if this ever goes back to `?v=`.
- **A stale version is served, not cached.** `/v/<version>/...` answers
  `public, max-age=31536000, immutable` only when the version is the running
  build's; any other version gets the current file with `no-cache`, so a page
  left open across a deploy never pins new content under its old URL. The plan's
  `Has("v")` check would have. Bare `/css/`, `/js/`, `/static/` and `/images/`
  answer `no-cache`.
- **`RoomPageData.Version` and `hubVersion()` stay.** `room.templ` reads
  `Version` into `data-version` for the socket's version check. Only
  `RoomPageData.Bundle()` was removed; `room.templ` passes `/static/room.js`
  and `Base` versions it.
- **`data-journal-editor-src` is versioned as well** (`journal-entry.templ`), so
  the editor the room window loads with `import()` is cached like the rest.
- **There was no `readVersion` test to move.** `internal/buildinfo` holds
  `Version` and `readVersion` and nothing else.
- **`streamObject` takes the getter and the key separately**
  (`get func(context.Context, string) (io.ReadCloser, int64, error), key, etag, cache string`)
  so the log lines keep the key. `ETag` is set only on 200 and 304, like
  `Cache-Control`.
- **An abandoned fetch answers 404, not nothing.** Returning without writing
  makes `net/http` send an empty 200, so the cancelled branch writes the same
  uncacheable 404 as a storage failure.
- **Policy constants.** `cacheImmutable` and `cacheRevalidate` in
  `controllers/assets.go` replace the five literals.
- **Tests as built:** `controllers/stream_test.go` (five cases plus headers),
  `templ/layouts/layouts_test.go` (every local `src`/`href` in Base and Share
  starts `/v/<version>/`, plus the relative-import test), and in
  `routes_test.go` the cache policy per path, a versioned and a bare path
  serving the same bytes, and nothing outside the four directories served under
  `/v/`. `room-page_test.go`, `journal_test.go` and `pages_test.go` now expect
  `layouts.Asset(...)` URLs.
- **Stop point as run:** steps 1 and 2 expect `/v/<version>/` URLs with the
  immutable policy for every stylesheet and module including `events.js`, and
  `toast.js` fetched once; the `no-cache` check moved to
  `/images/default-avatar.webp` and to `/v/stale/css/app.css`. Step 4 also
  expects no `ETag`.

### 1.1 Cache headers only on success (B1)

Where: `server/internal/controllers/assets.go:170` `streamImage`, and its five
callers: `GetImage` (`assets.go:167`), `GetMapTile` (`map-tiles.go:106`), the
journal image handler (`journal-images.go:129`), the shared journal image
handler (`share.go:193`) and `sharePortrait` (`share.go:249`).

Change:

- Replace the body of `streamImage` with a package function that takes the
  cache policy and a fetcher, so the policy is a parameter rather than a header
  the caller set earlier:

  ```go
  func streamObject(w http.ResponseWriter, r *http.Request, etag, cache string, get func(context.Context) (io.ReadCloser, int64, error))
  ```

  Order inside: set `ETag`; on an `If-None-Match` hit set `Cache-Control` and
  write 304; call `get`; on error, if the request context is done return
  without writing anything, otherwise log and write 404 with no
  `Cache-Control`; on success set `Cache-Control`, `Content-Type`,
  `Content-Length`, 200, copy.
- `streamImage` stays as the method callers use and becomes one line that
  passes `a.Storage.Get` as the fetcher.
- Every caller stops setting `Cache-Control` itself and passes it instead:
  `private, no-cache` for `GetImage` and `sharePortrait`,
  `private, max-age=31536000, immutable` for tiles and journal images. The
  `X-Robots-Tag` lines in the share handlers stay where they are.

Tests:

- New `server/internal/controllers/stream_test.go`: a table over `streamObject`
  with a fetcher that fails, a fetcher that succeeds, an `If-None-Match` hit and
  a cancelled context. Assert status, body, and that `Cache-Control` is present
  on 200 and 304 and absent on 404. This is the test the review said would have
  caught B1.
- `map-tiles_test.go:161` `TestAMissingTileIsAnEmptyBodyAndNoCacheHeader`
  already covers the missing-row path and stays. The "row exists, object
  missing" path at handler level needs a fakeable `Storage`, which arrives in
  Phase 8; note it there.

### 1.2 Versioned static assets (B2)

Where: `server/routes.go:251` the `files` handler; `server/internal/hub/version.go`;
`server/templ/layouts/base.templ:11` to `:31`; `share.templ:20` to `:22`;
`server/templ/pages/clerk.templ:27`; `server/templ/pages/room.go:55` `Bundle`;
`server/internal/controllers/room-page.go:185` `hubVersion`.

Change:

- Move `Version()` and `readVersion` into a new package `internal/buildinfo`
  with their test. `hub.New` reads `buildinfo.Version()` where it reads
  `Version()` today (`hub.go:79`). `hub.Version()` the package function goes;
  `(*Hub).Version()` stays because the socket frames use it.
- Add `layouts.Asset(path string) string` in `server/templ/layouts/layouts.go`
  returning the path plus `?v=` plus `buildinfo.Version()`.
- Apply it to every local `href` and `src` in `base.templ` and `share.templ`,
  to `/js/clerk-mount.js` in `clerk.templ`, and inside `Base`'s
  `for _, src := range scripts` loop so the per-page scripts (`/js/room.js`,
  `/js/music-upload.js`, `/js/zone-detect.js`, `/js/account-name.js`,
  `JournalEditorScript`, the room bundle) are versioned without touching their
  callers. The Clerk CDN URLs are not local and are left alone.
- `RoomPageData.Bundle()` returns `/static/room.js` with no query; `Base`
  versions it. Remove `RoomPageData.Version` and `hubVersion()` if nothing else
  reads them; grep first.
- `files` in `routes.go`: when `r.URL.Query().Has("v")` set
  `Cache-Control: public, max-age=31536000, immutable`, otherwise
  `Cache-Control: no-cache`. The second branch is what the bare module imports
  inside `public/js` (`./events.js`, `./modal.js`, `./toast.js`) get: one
  conditional request per module per page load, answered 304 by
  `http.FileServer` from the file's modification time.
- Note for the record: the Docker image copies `server/` without `.git`, so in
  that build `buildinfo.Version()` is `dev-` plus a timestamp fixed at process
  start. Every restart busts the cache, which is correct, and it is already how
  the socket's version check behaves.

Tests:

- `server/routes_test.go`: `GET /css/app.css` answers `Cache-Control: no-cache`;
  `GET /css/app.css?v=x` answers the immutable policy.
- `server/templ/layouts/layouts_test.go` (new): render `Base` with two extra
  scripts and assert, with a regular expression over the output, that every
  `src` or `href` beginning `/css/`, `/js/` or `/static/` carries `?v=`.

### 1.3 404 log level (B10)

`routes.go:265` `notFound` and `middleware/fragment.go:23` `FragmentNotFound`
log at `Warn`. Change both to `Info`. No test.

### Stop point 1

Run:

```
make check
```

Expect: green, including the two new test files.

Manual, with `make run` and a signed-in browser:

1. Open `/rooms`. In the browser's network panel, check `app.css` and
   `content-modal.js`. Expect: both URLs end in `?v=` plus a version and carry
   `cache-control: public, max-age=31536000, immutable`.
2. In the same panel find `events.js` (a bare module import). Expect: no `?v=`,
   `cache-control: no-cache`, and on a second load a 304.
3. Open a room whose map is tiled and check one tile request. Expect: 200 with
   `cache-control: private, max-age=31536000, immutable`.
4. Stop the server, set `R2_BUCKET` in `server/.env` to a bucket that does not
   exist, start it again, and request the same tile URL with `curl -i` and the
   session cookie. Expect: 404, an empty body, and no `cache-control` header at
   all. Restore `R2_BUCKET`.
5. Request a bogus path such as `/nothing-here`. Expect: the log line for it is
   at `INFO`, not `WARN`.

## Phase 2. Closed rooms

### 2.1 Hide what a closed room cannot do (B3, view side)

Where: `server/templ/pages/room.go:184` `Menus()`; `server/templ/pages/room.templ`
lines 69 to 110.

Change:

- In `Menus()`, when `d.Closed` return only `d.roomMenu()` and `helpMenu()`,
  plus `d.debugMenu()` when `d.Debug`. A closed room is reachable by the GM
  only (`roomMember` turns players away), and the Room menu already carries
  "Reopen room", "Player List" and "Back to rooms".
- In `room.templ`, wrap `roomPawnMenu`, the `data-table-menu-host` div,
  `roomRemoveKey` and `roomToolbar` in `if !data.Closed`. `roomOverlay` and
  `roomWindowTemplate` stay, because the Player List window needs the template.
- Check `server/js/room/main.ts` for any `querySelector` on the toolbar or pawn
  menu that assumes the element exists; the closed page already has no canvas,
  so the socket path is guarded, but the toolbar wiring may not be.

### 2.2 Map hub errors to the right status (B3, handler side)

Where: `server/internal/controllers/room-socket.go:147` `rejectCommand`;
`server/internal/hub/hub.go:18` `errGone`, `:113` `Dispatch`, `:365` `resolve`;
`server/internal/hub/persist.go:18` `ErrNoRoom`.

Change:

- Export `ErrGone`.
- In `resolve`, replace the `view(...)` call with `h.room(ctx, roomID)` followed
  by an `ask` for the state clone, so a closed room yields `ErrNoRoom` from the
  store rather than `errGone`. `ErrGone` is then only ever "the actor unloaded
  under the command", which is what the retry loop in `Dispatch` is for.
- In `rejectCommand`, after the existing `*room.Error` branch:
  `hub.ErrNoRoom` becomes
  `htmx.Error(w, "Room Closed", "This room is closed. Reopen it from the Room menu to use the tabletop.", http.StatusNotFound)`;
  `hub.ErrGone`, `context.Canceled` and `context.DeadlineExceeded` become a 503
  with a "try again" message and a `Warn` log; everything else keeps the
  `Error` log and `htmx.ServerError`.

### 2.3 Nits in the same files

- `hub.go:117` `for range 2` becomes `for range reloadRetries` with
  `const reloadRetries = 2` next to `ErrGone`.
- `room/resolve.go:62` `gone` becomes exported `room.IsGone`; `hub.go:168`
  `gone` is deleted and its callers use `room.IsGone`.

Tests:

- `server/templ/pages/room_test.go` (new or existing): for a GM with
  `Closed: true`, `Menus()` labels are exactly Room and Help; for an open room
  Tabletop, Fog and Initiative are present.
- `room-socket_test.go`: `rejectCommand` with `hub.ErrNoRoom` writes 404 and an
  `HX-Trigger` alert; with `hub.ErrGone` writes 503; with an unknown error
  writes 500.
- `internal/hub`: with a fake `Store` whose `Load` returns `ErrNoRoom`,
  `Dispatch` of a `Resolver` command returns `ErrNoRoom`, not `ErrGone`.

### Stop point 2

Run `make check`.

Manual:

1. As a GM, open a room in two tabs. In tab A, Room menu, Close room, confirm.
2. In tab A after the reload, expect the menu bar to show Room and Help only
   (plus Debug in development). Expect Player List to open as a window and
   Reopen room to work.
3. In tab B, which still shows the open room, choose Tabletop, Clear tabletop,
   confirm. Expect the alert modal with the "Room Closed" heading. Expect no
   `Failed to dispatch a room command` line in the server log.
4. In tab B, choose Initiative, Sync tracker. Expect the same alert.
5. Reopen the room from tab A and confirm tab B works normally after a reload.

## Phase 3. Operations

Six small changes to how the process behaves. None touch markup.

### 3.1 Map pixel cap and a memory limit (B6)

Where: `server/internal/controllers/assets.go:35` `maxMapPixels`;
`server/internal/controllers/assets_test.go:298`; `server/.env.example`;
`compose.yaml`.

Change:

- `maxMapPixels = 60_000_000`. The alert text is built from the constant, so
  it reads "60 megapixels" with no other edit. Grep the templ files for "150"
  in case any copy repeats the number.
- `TestTheMapCapTakesWhatTheDecodedCapCannot`: the accepted map becomes
  9000 by 6000 (54 MP, still above the 40 MP decoded cap), the 20000 by 20000
  refusal stays, and the alert assertion looks for "60 megapixels".
- Add `GOMEMLIMIT=800MiB` to `server/.env.example` (no comment line). The Go
  runtime reads it directly; compose already passes `server/.env` through.
- Add `mem_limit: 1g` to the `tabletopper` service in `compose.yaml`. If
  production runs somewhere else, set the same pair there.

Why these numbers: the tiler decodes one map at a time (a single `pass` loop
claims and tiles sequentially), so its worst case is one 60 MP PNG at four
bytes a pixel plus the next pyramid level and sixteen encoders' buffers. The
upload handlers can hold two decoded 40 MP images at once on top of that.

| Load | Approximate peak |
| --- | --- |
| Tiler, 60 MP PNG | 300 to 350 MB |
| Two concurrent 40 MP upload decodes | 320 MB |
| Everything else | under 100 MB |

### 3.2 Requeue a tiling job on shutdown (B8)

Where: `server/internal/tiling/job.go:172` `abandon`; `tiling.go:44` `Maps`;
`server/main.go` shutdown sequence.

Change:

- In `abandon`, when `ctx.Err() != nil`, instead of returning: build a context
  with `context.WithoutCancel(ctx)` and a ten-second timeout, call `discard`
  on it to drop the partial generation, then `RequeueStrandedTilingJob` with
  the asset ID and the lease. Confirm that query sets `tile_state` back to
  `pending` and clears the lease without counting an attempt; if it counts one,
  add a sibling query that does not.
- `Maps` returns a `*Worker` with a `Shutdown(ctx)` that waits for the in-flight
  pass to finish. `main.go` calls it in the shutdown sequence alongside
  `rooms.Shutdown`, within the same five-second budget, so the process does not
  exit before the requeue lands.
- If the discard fails inside the budget, log that the generation is orphaned,
  the same wording `publish` already uses for a superseded generation.

Tests, in `server/internal/tiling` with the existing fake `rows` and `bucket`:
cancel the context during `build`; expect `RequeueStrandedTilingJob` called with
the lease, `FailMapTiling` not called, and the generation prefix deleted.

### 3.3 Name the expired-session close (B7)

Where: `server/internal/hub/conn.go:149`; `server/js/room/socket.ts:40`
`ENDED_REASONS`; `server/js/room/main.ts:255`; `server/js/room/exit.ts`.

Change:

- `const reasonExpired = "expired"` beside the other reasons; `conn.go:149`
  passes it instead of the empty string. The membership check fails when the
  session is gone, when the seat was cleared, or when the room closed, and all
  three end with the viewer being sent to the rooms page, so one name is enough.
- Add `"expired"` to `ENDED_REASONS`.
- `exit.ts` gains `leaveExpired()` that stores a pending alert ("Signed out",
  "Your session ended. Sign in again to rejoin the room.") and assigns
  `/rooms`; `main.ts`'s status handler calls it for `"expired"`. `/rooms`
  redirects to sign-in when there is no session, so one destination covers
  both the signed-out and the unseated case.

Tests: `server/js/room/socket.test.ts`, a close with reason `expired` marks the
socket ended and schedules no reconnect. If `internal/hub` has a conn test
harness, add the server half; if not, the manual step covers it.

### 3.4 Configure logging (S6)

Where: `server/main.go` `run`, immediately after `config.Load`.

Change: when `cfg.Development()` install `slog.NewTextHandler` on stderr at
`Debug`; otherwise `slog.NewJSONHandler` at `Info`; `slog.SetDefault` either
way. Five lines. Every `slog.Debug` in the codebase becomes live in
development.

### 3.5 A configured public URL for share links (security note)

Where: `server/internal/config/config.go`; `server/internal/controllers/share-form.go:47`
`shareLink` and `:57` `describeShare`; `server/.env.example`.

Change: add `PublicURL` from `PUBLIC_URL`, trimmed of a trailing slash,
optional. `shareLink` takes the base as a parameter and uses it when set,
otherwise the current `X-Forwarded-Proto` and `r.Host` derivation.
`describeShare` becomes a method or takes the base from the caller.

Test: `share-form_test.go`, `shareLink` with and without a base.

### 3.6 HSTS (security note, only if decision 4 says so)

Where: `server/internal/middleware/headers.go`.

Change: when the request arrived over TLS or carries
`X-Forwarded-Proto: https`, set
`Strict-Transport-Security: max-age=31536000; includeSubDomains`. Add a test in
`middleware` for both branches.

### 3.7 Write the capability-URL decision down (security note)

Add one short paragraph to `README.md` saying that `/assets/images/{id}` and
the tile routes require a session but not ownership, that this is deliberate so
players can see the GM's maps and tokens, and that the ULID is the capability.
No code.

### Stop point 3

Run `make check`. No SQL changed, so `make db` is not needed.

Manual:

1. Upload a 9000 by 6000 PNG as a map. Expect: accepted, tiled, visible in a
   room. Upload a 12000 by 9000 PNG. Expect: the alert "Images must be 60
   megapixels or fewer."
2. While the first map tiles, run `docker stats`. Expect: the container stays
   under the 1 GB limit and the server keeps answering other requests.
3. Upload a large map again and, as soon as the card shows it working, run
   `docker compose restart tabletopper`. Expect: after the restart the log shows
   the job requeued, the card returns to pending and then ready within the
   build time plus a few seconds, not fifteen minutes.
4. Join a room as a player in tab A. In tab B on the same browser, sign out.
   Expect: within the membership recheck interval, tab A navigates to the
   sign-in page and shows the "Signed out" alert once signed in again, and
   the server log shows one close, not a reconnect every fifteen seconds.
5. With `ENV` set to development, open a page and abandon an image fetch
   (navigate away mid-load). Expect: a `DEBUG` line such as "Image fetch
   abandoned by the client". Set `ENV` to anything else, restart. Expect: JSON
   log lines and no `DEBUG`.
6. Set `PUBLIC_URL=https://example.test`, restart, open a character's share
   dialog and create a link. Expect: the link begins with that origin. Unset
   it. Expect: the link uses the browser's host as before.
7. If 3.6 was done: `curl -I -H "X-Forwarded-Proto: https"` against the
   server. Expect: the HSTS header. Without the header, expect none.

## Phase 4. Deletion

One mechanism for removing objects from R2, replacing five hand-rolled orders of
operations. Assumes decision 1.

### 4.1 The purge queue (S3)

Migration `db/migrations/<timestamp>_object_purge.sql`, in the shape of its
neighbours:

```sql
CREATE TABLE object_purge (
  id varbinary(16) NOT NULL,
  object_key varchar(1024) NOT NULL,
  is_prefix tinyint(1) NOT NULL DEFAULT 0,
  attempts tinyint unsigned NOT NULL DEFAULT 0,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_object_purge_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

Queries in a new `server/sql/objects.sql`: `EnqueueObjectPurge :exec`,
`ListObjectPurges :many` ordered by `created_at` with `LIMIT 100`,
`DeleteObjectPurge :exec`, `RecordObjectPurgeAttempt :exec`. Run `make sqlc`.

Sweeper `server/internal/sweep/objects.go`: `Objects(ctx, q, store)` on a
one-minute ticker with a first pass at boot, in the same shape as
`sweep/journal_images.go`. For each row: `Delete` or `DeletePrefix` according to
`is_prefix`; on success delete the row; on failure record the attempt and leave
it for the next pass, logging at `Error` once per pass with the count. The
store parameter is a two-method interface (`Delete`, `DeletePrefix`), as
`sweep.Objects` in `accounts.go` already does for `DeletePrefix`. Wire it in
`main.go`.

Controller helpers in `server/internal/controllers/assets.go`:
`enqueuePurge(ctx, q *queries.Queries, keys ...string) error` and
`enqueuePurgePrefix(ctx, q, prefix string) error`, both for use inside an
`a.tx` closure.

### 4.2 Reverse every delete (B4)

Each handler drops its pre-transaction `Storage` call and enqueues inside the
transaction instead:

- `characters.go:53` and `:59` `DeleteCharacter`: list the journal image keys
  and the portrait key inside the transaction (the list query moves in with
  them), enqueue, then the existing row deletes in their existing order.
- `monsters.go:360` `DeleteMonster`: enqueue `monster.FilePath` inside the
  transaction.
- `assets.go:701` `DeleteMap`: an `a.tx` with `DeleteAsset` and
  `enqueuePurgePrefix(storage.MapPrefix(...))`.
- `library-assets.go:290` `deleteLibrary`: an `a.tx` with `DeleteAsset` and
  `enqueuePurge(row.FilePath)`.
- `assets.go:844` `discardAsset`: becomes a transaction on
  `storage.CleanupContext(ctx)` that deletes the row and enqueues the key or
  prefix; the `remove func` parameter goes and its nine call sites pass the key
  or prefix instead. `discardAssetRow` stays as the no-object variant.
- `sweep/journal_images.go`: the tail becomes "delete the row, enqueue the
  key" per image, and the function no longer needs the store.
- `sweep/accounts.go:127` may enqueue `storage.UserPrefix(user)` instead of
  calling `DeletePrefix` itself, which makes account purge a pure DB job with
  free retries. Optional; do it if the rest went smoothly.

After this, no request handler calls `Storage.Delete`, `DeleteMany` or
`DeletePrefix`, and an R2 outage cannot turn a delete into a 500.

### 4.3 Small ones (B10)

- `sql/journals.sql:25` `CountJournalImages`: add to the join condition
  `AND (a.detached_at IS NULL OR a.detached_at > sqlc.arg(recent))`, with the
  caller in `journal-images.go:43` passing now minus one hour. Images removed
  from an entry stop counting after an hour; images uploaded during an editing
  session still count. Shorter than the 24-hour sweep grace on purpose.
- `sql/assets.sql:55` `UpdateAssetName` becomes `:execresult`; the two rename
  handlers (`library-assets.go:270`, `assets.go:820`) check rows affected and
  answer `htmx.NotFound` on zero.
- `sql/user.sql:12` `SetUserAvatar` becomes `:execresult`; `assets.go:409`
  treats zero rows as the account having gone and discards the asset.
- `storeMap` (`assets.go:639`) and `attachMonsterImage` (`assets.go:587`) pass
  the filename through `assetName` like every other upload path.

Tests:

- `server/internal/sweep/objects_test.go`: fake rows and a fake deleter;
  objects deleted then rows; a failing deleter leaves the row and records an
  attempt; prefix rows call `DeletePrefix`.
- `characters_test.go:60` and `:90`, `monsters_test.go:222`, the
  `library-assets_test.go` delete cases: update the expected statement lists to
  include the `INSERT INTO object_purge` rows and assert no statement runs
  outside the transaction. Because tests construct `App` with a nil `Storage`,
  the handlers reaching the end without a panic is itself proof that no R2 call
  remains on the request path.
- `journal-images_test.go`: the count query carries the `detached_at` guard and
  the handler passes a cutoff about an hour old.
- A rename of a missing asset answers 404.

### Stop point 4

Run:

```
make db
make sqlc
make check
```

Manual:

1. Create a character with a portrait and a journal entry with two images.
   Delete the character. Expect: the toast at once, and
   `SELECT object_key, is_prefix FROM object_purge` shows three rows.
2. Wait a minute. Expect: the table is empty and the portrait URL answers 404.
3. Delete a tiled map. Expect: one row with `is_prefix = 1`; after the sweep,
   a tile URL for it answers 404.
4. Point `R2_BUCKET` at a bucket that does not exist and restart. Delete a
   token. Expect: the toast, the token gone from the library at once, a purge
   row whose `attempts` climbs once a minute, and an `ERROR` line per pass.
   Restore the bucket and restart. Expect: the row drains on the next pass.
5. Rename a map in one tab, delete it in another, then submit the rename.
   Expect: the "no longer exists" alert, not a success toast.
6. Upload a map whose filename is longer than the asset name limit. Expect: the
   card shows the truncated name, the same as a token upload would.

## Phase 5. Actor performance

### 5.1 Two fewer clones and no whole-state compare (B5)

Where: `server/internal/room/derive.go:10` `Derive`;
`server/internal/hub/actor.go:313` `broadcast` and `:264` `exec`.

Change:

- `Derive`: for `RoleGM`, use `before` and `after` as they are;
  `Project(RoleGM)` is `Clone()` and `Derive` only reads. For the player role
  keep the projections. Before doing this, confirm that every change payload
  already copies what it carries: the `diff` helpers clone through their
  `clone` field, `TilesStamped` builds a new slice of comparable values,
  `RoomUpdated` carries a value struct, and `TableUpdated` carries
  `TableSettings`. Open `state.go:35` and check `TableSettings` has no slice,
  map or pointer field; if it has one, clone it in `Derive` for that change.
- `broadcast`: remove the `reflect.DeepEqual` over the whole state. Derive the
  GM changes first; if there are none return `nil` without touching `dirty` or
  `changes`; otherwise mark dirty, send the GM frame, then derive and send the
  player frame. The GM projection is the whole observable state, so an empty GM
  diff is exactly "nothing changed".
- `resync` keeps its `DeepEqual`; it runs once per scene load, not per stroke.

Net effect per committed command: three clones instead of five (the rollback
copy and the two player projections) and no whole-state compare.

### 5.2 Nit considered and dropped

`cloneSlice` in `room/command.go` returns an empty non-nil slice for nil input
and `slices.Clone` returns nil. Several change payloads serialise the result
(`PaletteUpdated` among them), and a `null` where the client expects an array
would throw in a `for ... of`. Keep `cloneSlice`. The review's suggestion is
withdrawn.

Tests:

- `server/internal/room/derive_test.go`: GM derivation output does not alias
  the after state (mutate `after` once `Derive` returns and assert the changes
  are unchanged); an unchanged state derives no changes for either role.
- `server/internal/hub`: a command whose `Apply` changes nothing leaves `dirty`
  false and sends no frame; one that changes a GM-only field sends a GM frame
  and no player frame.
- A `BenchmarkDeriveStrokeExtend` over a state with a few thousand stroke
  points, kept for the next time this is touched.

### Stop point 5

Run `make check`, and `go test -race -count=3 ./internal/hub/ ./internal/room/`
from `server/`.

Manual, with a GM tab and a player tab in the same room:

1. Draw free-hand for ten seconds without lifting. Expect: the player sees the
   stroke grow smoothly; the GM's Debug, Events window shows one
   `strokes.*` change per chunk and nothing else.
2. Move a pawn, then drop it back on the same cell. Expect: a frame for the
   move and no frame for the no-op drop.
3. Hide a pawn from players. Expect: a GM frame; the player's Events window
   shows a removal, nothing more.
4. Change a layer's GM map, stamp a tile, clear fog, play a track, roll
   initiative. Expect: each reaches the player as before.
5. Close the GM tab and reopen the room. Expect: the state reloads from the
   snapshot with everything from steps 1 to 4 intact, which proves `dirty` was
   still set when something changed.

## Phase 6. Game rules out of the view package

### 6.1 `internal/dnd` (S1)

Where: `server/internal/controllers/rules.go` (XP thresholds, proficiency,
ability modifiers, challenge rating lookup);
`server/templ/pages/character-options.go` (sizes, alignments, normalisers,
labels); `monster-options.go` (creature types, challenge ratings and their XP
and proficiency); `bonuses.go` (proficiency states, skills, saving throws,
passive and spell save bases); `limits.go`; `server/internal/room/state.go:255`
`Size`.

Change, in this order so the tree builds at every step:

1. Create `internal/dnd` with `Option`, the size, alignment and creature type
   tables, `ChallengeRating` and its table, the proficiency states, the skill
   and saving-throw entries, the XP thresholds, `LevelFromXP`,
   `ProficiencyBonus`, `AbilityModifier`, the `Normalize*` and `*Label`
   functions, and the rule constants that are not tied to `room` or `share`
   (`AbilityScoreLimit`, `CharacterXPLimit`, `SpellSaveDCBase`,
   `PassiveScoreBase`). `dnd` imports nothing from the project.
2. Turn every moved identifier in `pages` into an alias or one-line forwarder to
   `dnd`, so the templates compile unchanged.
3. Point `controllers` and `export` at `dnd` directly and delete the
   forwarders that then have no caller. What remains in `pages` is view data:
   `BonusRow`, `Derived`, `StatBlock`, the monster action sections and the
   templ components.
4. `room.Size.Values()` returns the `dnd` size values so the two cannot drift.
   Run `make protocol` and confirm `protocol.ts` is unchanged.

After this the import graph is `dnd` at the bottom, `room` and `pages` above
it, `controllers` and `export` above those, and nothing imports `pages` except
`controllers` and `export`.

### 6.2 Go nits

- `uploadProblem` (`assets.go:220`) gains an `Error() string` method and
  `errNotMultipart` is checked with `errors.Is` in `monsters.go:266` and
  `room-pawns.go:258`.
- `requeueMap` (`assets.go:787`) takes `ctx` first; `sharePortrait`
  (`share.go:249`) becomes a method on `*App`.
- `share-monster.go:156` uses `ulid.Make()` instead of a private monotonic
  entropy source.
- `characters.go:420` `parseFeatures` returns a nil slice when empty, once the
  template is confirmed to range over nil without special-casing it.
- `room/code.go:20` stays as it is; `rand.Read` cannot fail since Go 1.24 and
  the code's alphabet is not the one `rand.Text` produces.

Tests: move the rule tests with the code (`monster-rules_test.go` and the
`pages` tests that cover the tables); add `TestRoomSizeValuesMatchDnd`.

### Stop point 6

Run `make check` and `make protocol`, then `git status`. Expect: no change to
`server/js/room/protocol.ts` or any generated file.

Manual, recording values before the phase and comparing after:

1. Open an existing character's sheet and note the proficiency bonus, two
   skill totals, passive perception and spell save DC. Expect: identical after.
2. Open the monster editor, check the size, alignment, creature type and
   challenge rating dropdowns, pick a CR. Expect: the same options in the same
   order, the same XP and proficiency shown in the stat block.
3. Export a character to markdown before and after. Expect: a byte-identical
   file.
4. Spawn a Large and a Gargantuan monster on a table. Expect: footprints of 2
   and 4 cells as before.

## Phase 7. Boilerplate

### 7.1 One share dialog controller (S4)

Where: `character-share.go`, `journal-share.go`, `monster-share.go` (460 lines
together); the pattern to copy is `assetKind` in `library-assets.go`.

Change: a `shareKind` struct with the heading, blurb, the action path builder,
and three functions wrapping `Get`, `Insert` and `Delete` for that resource.
`shareFragment`, `createShare` and `revokeShare` take the kind; the nine public
handlers become one-liners. The routes in `routes.go` do not change. The share
tests (`character-share_test.go`, `journal-share_test.go`, `monster-share_test.go`)
keep their assertions and run against the collapsed handlers.

### 7.2 Fragment and path helpers (S4)

- `renderFragment(w, r, c)` sets `Content-Type: text/html; charset=utf-8` and
  calls `render`. Replace the 79 pairs.
- `pathID(w, r, name, what string) (ulid.ULID, bool)` parses
  `r.PathValue(name)` and answers `htmx.NotFound(w, what)` on failure. Replace
  the 28 `ulid.Parse(r.PathValue("id"))` pairs and their siblings for other
  names.

Mechanical; `make check` is the test. Do it with search and replace, one
handler file at a time, and read each diff.

### 7.3 `sweep.Every` (S5)

Where: the six ticker loops in `session/cleanup.go`, `tiling/tiling.go`,
`sweep/accounts.go`, `sweep/journal_images.go`, `sweep/shares.go`,
`sweep/music_uploads.go`, plus `sweep/objects.go` from Phase 4.

Change: `sweep.Every(ctx, name string, interval time.Duration, fn func(context.Context))`
runs `fn` once at start and on each tick, and logs `name` stopped on
`ctx.Done`. `session` and `tiling` import `sweep`; check for a cycle first
(`sweep` imports `queries` and `storage`, neither of which import `session` or
`tiling`). `tiling.Maps` keeps its `Worker` return from Phase 3 and uses
`Every` inside.

### 7.4 Membership helpers

Where: `room-page.go:214` `roomMember` and `:191` `loadRoomMember`,
`room-table.go:409` `gmTable`, `room-palette.go:100` `dispatchRoom`,
`scenes.go:330` `sceneRoom`, `room-pawns.go:365` `spawnRoom` and `:686`
`pawnActor`.

Change: two helpers. `member(w, r) (row, role, ok)` answers the redirect or
404 itself; `gm(w, r) (row, ok)` calls `member` and additionally requires the
GM role. The others become thin wrappers or disappear. Move the
`Sessions.LeaveRoom` side effect out of `roomMember` into the one caller that
needs it (`RoomPage`), so a function named for reading does not write.

### 7.5 Comments in `public/js` (decision 3)

Strip the roughly 600 comment lines across the 16 files, including JSDoc
blocks. Keep the behaviour byte for byte otherwise; run `make js-test` and
`make check`. If decision 3 chose the exception instead, add the sentence to
`CLAUDE.md` and skip this item.

### Stop point 7

Run `make check`. Also `grep -rn "Content-Type\", \"text/html" server/internal/controllers | wc -l`.
Expect: one hit, inside `renderFragment`.

Manual:

1. For a character, a journal entry and a monster: open the share dialog,
   create a link with a password and a seven-day expiry, copy it, open it in a
   private window, enter the password. Expect: the page renders. Revoke the
   link. Expect: the private window now gets a 404 on reload.
2. Open the Layers, Grid, Player List and Monster Manual windows in a room.
   Expect: each loads; the response headers of their fragment requests carry
   `content-type: text/html; charset=utf-8`.
3. Request `/characters/not-a-ulid` and `/monsters/not-a-ulid` with a session.
   Expect: the "no longer exists" alert on an htmx request, a 404 otherwise.
4. Restart the server and read the first page of the log. Expect: one "first
   pass" line per sweeper, and on SIGTERM one "stopped" line per sweeper.
5. As a player, open the room page for a room that was closed. Expect: the
   redirect to `/rooms/join` as before, and the session no longer names the
   room.

## Phase 8. Interfaces

This phase is best done one feature at a time, each with the next feature that
touches it. The first two items unblock the test the review asked for and
remove the nil guards; the rest is a pattern to follow.

### 8.1 `Hub` as an interface (S2)

Where: `server/internal/controllers/app.go` `App.Hub`; the 29
`if a.Hub == nil` guards.

Change: a `Hub` interface in `controllers` listing the nineteen methods the
handlers call (`Dispatch`, `Table`, `Pawn`, `Players`, `Music`, `Initiative`,
`ForgetAsset`, `Export`, `Autosave`, `Version`, `SyncCharacter`, `Serve`,
`Rolls`, `Notify`, `ForgetMonster`, `ForgetCharacter`, `Debug`, `Close`,
`CharacterPawn`). `*hub.Hub` satisfies it. A `nopHub` in the test files returns
"not loaded" from every view method. Tests construct `App{Hub: nopHub{}}`;
`main.go` is unchanged. Delete the 29 guards.

### 8.2 `Storage` as an interface (S2, and the B1 handler test)

Change: an `objects` interface in `controllers` with the methods the handlers
call (`Get`, `Put`, `PutReader`, `Peek`, `Size`, `PresignPut`, `PresignGet`,
the `Upload*` helpers). After Phase 4 the delete methods are no longer in the
list. A map-backed fake in the tests. Then add the handler-level test deferred
from Phase 1: `GetMapTile` with a pyramid row present and the object missing
answers 404 with no `Cache-Control`.

### 8.3 Feature stores (S2)

Pattern, applied per feature as that feature is next touched: a small
interface in the handler's file listing only the query methods it calls
(`shareStore` after Phase 7 is three methods per kind; `tileStore` is one
method), satisfied by `*queries.Queries`, with a map-backed fake in the test.
Each store retires the `recordingDB` or `oneRowDB` usage for that feature. Stop
when `onerow_db_test.go`, `room_db_test.go` and
`middleware/session_db_test.go` have no remaining callers and can be deleted.

### Stop point 8

Run `make check`. Expect: `grep -rn "a.Hub == nil\|a.Hub != nil" server/internal/controllers`
finds nothing.

Manual smoke test, since this phase changes wiring and not behaviour:

1. Sign in, create a room, join it from a second account, spawn a monster, move
   a pawn, roll dice, play a track, close the room.
2. Upload a token, an avatar and a map; replace the map; delete all three.
3. Open and tile a map, request a tile, request a tile for a bogus
   coordinate. Expect: 200 then 404 with no cache header.

## Not planned, with reasons

- **B9 sheet sync race.** Tens of milliseconds wide and self-correcting on the
  next write-through. Revisit only if a player reports HP jumping back.
- **Scene cap under concurrency.** The 200-scene cap is advisory and a user
  racing their own double-click gets 201 scenes at worst.
- **`cloneSlice` to `slices.Clone`.** Withdrawn; see 5.2.
- **`GetImage` type list.** `profile` and `character` are read by room member
  lists and pawns, so they stay. The README paragraph in 3.7 records why.
- **CSP.** A report-only policy is worth trying after Phase 3 is stable, but
  Clerk's scripts make it a project of its own.
