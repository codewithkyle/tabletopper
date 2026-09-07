# Phase 1: rooms as a resource, and the room shell

Read `plans/vtt-overview.md` first. Every rule in `CLAUDE.md` applies: no comments
in `.templ` files, fragments only under `/fragment/` and only as `GET`, three
modals and no fourth, no `HX-Trigger` by hand. This document is deleted when the
phase lands; nothing in code, migrations or the Makefile may reference it.

## End state

- The homepage's two dead links work. **Host Game** goes to `/rooms`, the GM's
  rooms. **Join Game** goes to `/rooms/join`.
- A GM creates a named room from a one-field dialog, lands on `/rooms/{id}`,
  and sees the room's four-character code, a lock toggle, a members panel, and
  an empty table region. The GM can close the room, reopen it with a new code,
  and delete it.
- A player enters a code (and optionally picks one of their characters), lands
  on the same `/rooms/{id}` page, and sees the same shell without the GM
  controls. The player can leave.
- Nothing is live. No socket, no hub, no canvas, no JavaScript beyond what the
  layout already loads. Every interaction is an htmx request against the
  database, using the patterns the characters feature already uses.

Everything below is HTTP and SQL. The phase is small on purpose: it settles the
URLs, the join flow, the schema, and the page every later phase mounts on.

## Decisions

1. **Rooms are persistent and named.** A room belongs to a GM and lives until
   they delete it. It is either *open* (has a code, players can join) or
   *closed* (no code, nobody can join, members are bounced). Reopening mints a
   new code. The snapshot that phase 3 adds makes a room worth keeping between
   sessions; an ephemeral room is just one the GM deletes afterwards.
2. **The room URL is `/rooms/{id}`**, a ULID. The code is only for joining.
   Codes are recycled when a room closes, so a code-based URL would break for
   the GM and keep working for the wrong room. The player reaches `/rooms/{id}`
   by redirect after a successful join.
3. **Membership is `sessions.room_id`.** The column exists, is selected by
   `GetSession`, and is already on `session.UserSession`; nothing writes it
   yet. A join sets `room_id` and `character_id` on the current session. A
   session is in at most one room. The GM is never a member: ownership is what
   admits the owner, and the role is derived from `rooms.owner_id`.
4. **The join endpoint is rate limited per user, not per code.** `share.Attempts`
   is keyed by the share token because a share request is unauthenticated and
   the only client address is one Cloudflare wrote. A join is authenticated, so
   the user id is a key the client cannot reset. Reuse `share.Attempts` with the
   user id as the key, held on `App` as `RoomJoinAttempts`, constructed in main.
5. **A locked room says it is locked.** The join form distinguishes "no open
   room has that code" from "that room is locked". This leaks that a code is in
   use, which the shares design avoids on purpose. A room code is not a bearer
   credential: knowing one admits you to a table where the GM can see you and
   kick you, and the rate limit bounds the enumeration. The better message for
   the friend who typed the right code wins.
6. **The snapshot lives on the `rooms` row.** Three columns, written by phase 3
   and read by nobody until then: `snapshot JSON NOT NULL DEFAULT (json_object())`,
   `snapshot_seq BIGINT UNSIGNED NOT NULL DEFAULT 0`, `snapshot_at DATETIME NULL`.
   An empty object means no snapshot yet. Every query against `rooms` names its
   columns, so nothing pulls the blob by accident.
7. **`rooms` is rebuilt with DROP and CREATE.** `grep -rn rooms server/sql` is
   empty; nothing has ever written the table. The existing
   `UNIQUE KEY ux_rooms_code_open (code, is_open)` allows exactly one open and
   one closed room per code, so closing a second room with a recycled code
   would collide. The rebuild makes `code` nullable and unique on its own,
   drops `is_open`, and defines open as `closed_at IS NULL`. One state, one
   column, one writer.
8. **No hub in this phase.** A hub that only tracked liveness would have
   nothing to track. Phase 3 introduces it with the socket.
9. **Code alphabet is `ABCDEFGHJKMNPQRSTUVWXYZ23456789`**, four characters,
   about 920,000 codes. It omits `I`, `L`, `O`, `0`, `1`. Input is trimmed and
   uppercased before validation, so a player can type lowercase.
10. **Joining without a character is allowed.** The character select offers the
    player's characters plus "No character". A player may join before they have
    made one.

## Migration

`db/migrations/20260907160000_rooms_reshape.sql`. House style: long prose
comments in which a capitalised lead clause states the decision and the
paragraph defends it, alternatives named and refused, the failure mode each
choice avoids spelled out. Cross-reference earlier migrations by timestamp only.

Up:

```sql
DROP TABLE IF EXISTS rooms;
CREATE TABLE rooms (
    id           VARBINARY(16) PRIMARY KEY NOT NULL,
    owner_id     VARBINARY(16) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    code         CHAR(4) CHARACTER SET ascii NULL,
    is_locked    TINYINT(1) NOT NULL DEFAULT 0,
    snapshot     JSON NOT NULL DEFAULT (json_object()),
    snapshot_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    snapshot_at  DATETIME NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at    DATETIME NULL,
    UNIQUE KEY ux_rooms_code (code),
    KEY idx_rooms_owner (owner_id, created_at)
);

ALTER TABLE sessions
    ADD INDEX idx_sessions_room_id (room_id);
```

Down: drop the sessions index, drop `rooms`, recreate it exactly as
`20260227175838` did.

Comments the migration owes, in the house style:

- Why DROP and CREATE rather than ALTER (never written; the unique key bug).
- Why `code` is nullable and the row is kept when it closes (persistent rooms;
  a closed room has no code so `UNIQUE (code)` permits any number of closed
  rooms; MySQL unique indexes allow multiple NULLs).
- Why there is no `is_open` (open is `closed_at IS NULL`; two columns that can
  disagree is the shares rule from `20260906190000`).
- Why the snapshot is three columns on this row and not a table (one row per
  room, overwritten, no history; every reader names its columns).
- Why `snapshot` is `NOT NULL DEFAULT (json_object())` (sqlc maps a nullable
  JSON column to `sql.NullString`, and every other JSON column in the schema
  is NOT NULL for the same reason; empty object means none).
- Why `snapshot_seq` is BIGINT UNSIGNED (the room's monotonic sequence; a
  column that has to be widened later is widened during an incident).
- Why `idx_sessions_room_id` exists (the members list and the close-room sweep
  both filter sessions by room).
- Column names avoid `status`, `list`, `table`, `tab`, `stack`, `swap`,
  because sqlc identifiers reach `.templ` files and Tailwind scans them.

Then `make db` with MySQL up (it regenerates `db/schema.sql`, which is sqlc's
input) and `make sqlc`. No `sqlc.yaml` change is needed: `rooms.owner_id`
matches `*.owner_id`, `rooms.id` matches `*.id`, `snapshot` takes sqlc's
default `json.RawMessage`.

## Queries

### `server/sql/rooms.sql` (new)

Every owner-scoped statement carries `AND owner_id = ?` in the WHERE. Zero
matched rows means not yours or not there, and both answer 404. Use
`:execresult` wherever the handler tells those apart; `ClientFoundRows` is on,
so `RowsAffected` reports matched rows.

| Name | Kind | Statement |
| --- | --- | --- |
| `CreateRoom` | `:exec` | `INSERT INTO rooms (id, owner_id, name, code) VALUES (?, ?, ?, ?)` |
| `ListRooms` | `:many` | `SELECT id, name, code, is_locked, created_at, closed_at FROM rooms WHERE owner_id = ? ORDER BY created_at DESC` |
| `GetRoom` | `:one` | `SELECT id, owner_id, name, code, is_locked, created_at, closed_at FROM rooms WHERE id = ?` (not owner-scoped: members read it too; the handler decides role) |
| `GetOpenRoomByCode` | `:one` | `SELECT id, is_locked FROM rooms WHERE code = ?` (a closed room has NULL code, so this cannot match one; selects nothing gated, like `GetShareByToken`) |
| `SetRoomLocked` | `:execresult` | `UPDATE rooms SET is_locked = ? WHERE id = ? AND owner_id = ? AND closed_at IS NULL` |
| `CloseRoom` | `:execresult` | `UPDATE rooms SET code = NULL, closed_at = NOW() WHERE id = ? AND owner_id = ? AND closed_at IS NULL` |
| `OpenRoom` | `:execresult` | `UPDATE rooms SET code = ?, closed_at = NULL, is_locked = 0 WHERE id = ? AND owner_id = ? AND closed_at IS NOT NULL` |
| `RenameRoom` | `:execresult` | `UPDATE rooms SET name = ? WHERE id = ? AND owner_id = ?` |
| `DeleteRoom` | `:execresult` | `DELETE FROM rooms WHERE id = ? AND owner_id = ?` |

The snapshot columns get no query in this phase.

### `server/sql/session.sql` (additions)

| Name | Kind | Statement |
| --- | --- | --- |
| `SetSessionRoom` | `:execresult` | `UPDATE sessions SET room_id = ?, character_id = ? WHERE hash = ?` |
| `ClearSessionRoom` | `:execresult` | `UPDATE sessions SET room_id = NULL, character_id = NULL WHERE hash = ?` |
| `ClearRoomSessions` | `:execresult` | `UPDATE sessions SET room_id = NULL, character_id = NULL WHERE room_id = ?` |
| `ListRoomMembers` | `:many` | `SELECT DISTINCT user_id, username, profile_image_url, character_id FROM sessions WHERE room_id = ? AND expires_at > NOW() ORDER BY username` |

Key on `hash` because `EndSession` does; the hash is what the store holds. The
members query reads sessions rather than users because `username` and
`profile_image_url` are already denormalised onto the session row, which
`session.go` documents as the intended read path. `DISTINCT` collapses one user
with two browsers into one member. Phase 3 replaces this list with the live one
from room state; the query stays as the initial render's fallback.

## Package changes

### `server/internal/session`

Two methods on `Store`, because `Store.q` is unexported and the package doc
says the store is the only code that touches the row:

```go
func (s *Store) JoinRoom(ctx context.Context, u *UserSession, roomID ulid.ULID, characterID *ulid.ULID) error
func (s *Store) LeaveRoom(ctx context.Context, u *UserSession) error
```

Each runs its statement, treats zero matched rows as an error (the session
ended under us), and mutates `u.RoomID` and `u.CharacterID` so the handler's
copy agrees with the row. No cookie change; the auth middleware reads the row
fresh on every request.

### `server/internal/room/code.go` (new package)

The package `internal/room` will hold the pure protocol core in phase 2. It
starts here with the code generator, which is equally pure.

```go
const CodeLength = 4
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
func NewCode() string                 // crypto/rand, rejection-sampled
func NormalizeCode(s string) string   // TrimSpace + ToUpper
func ValidCode(s string) bool         // length 4, every rune in the alphabet
```

`ValidCode` runs before any statement, exactly as `share.ValidToken` does: a
value that cannot name a row does not need a query to find that out.

### `server/internal/controllers/app.go`

Add `RoomJoinAttempts *share.Attempts` beside `ShareAttempts`, constructed in
`main.go` as `share.NewAttempts(10, time.Minute)`. Tests supply their own.

## Routes

Register in `server/routes.go` as a new block between the monster block and the
assets block, under a comment explaining the shell. Every pattern names a
method.

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /rooms` | `RequireSession` | `RoomsPage` | The GM's rooms, newest first, with an empty state. |
| `POST /rooms` | `RequireSession` | `NewRoomForm` | Create. Mints `ulid.Make()` and `room.NewCode()`, retries the insert on a duplicate-key error for the code (bounded, say five tries), then `htmx.Toast` and `htmx.Redirect` to `/rooms/{id}`. Validation errors follow `rejectNewCharacter`: 422 and `PanelFormErrors`. |
| `GET /rooms/join` | `RequireSession` | `JoinRoomPage` | Code field and character select. |
| `GET /rooms/join/{code}` | `RequireSession` | `JoinRoomPage` | Same page with the code prefilled, for shared links. Never joins on GET. |
| `POST /rooms/join` | `RequireSession` | `JoinRoomForm` | See the join flow below. |
| `GET /rooms/{id}` | `RequireSession` | `RoomPage` | The shell. See access rules below. |
| `POST /rooms/{id}/lock` | `RequireSession` | `LockRoom` | GM only. Sets `is_locked = 1`; answers with the re-rendered lock control (a mutation may return the row it changed). |
| `POST /rooms/{id}/unlock` | `RequireSession` | `UnlockRoom` | GM only. Mirror of lock. |
| `POST /rooms/{id}/close` | `RequireSession` | `CloseRoom` | GM only. In one `a.tx`: `CloseRoom` then `ClearRoomSessions`. `htmx.Toast` and `htmx.Redirect` to `/rooms`. |
| `POST /rooms/{id}/open` | `RequireSession` | `OpenRoom` | GM only. Mints a new code (same retry), `htmx.Redirect` to `/rooms/{id}`. |
| `POST /rooms/{id}/leave` | `RequireSession` | `LeaveRoom` | Member only; the `{id}` must equal the session's room or the answer is 404. `Store.LeaveRoom`, then `htmx.Redirect` to `/`. |
| `DELETE /rooms/{id}` | `RequireSession` | `DeleteRoom` | GM only, behind `hx-confirm` on the card. In one `a.tx`: `ClearRoomSessions` then `DeleteRoom`. Empty 200; the card is `hx-swap="delete"`. |
| `GET /fragment/room/new` | `Fragment` | `NewRoomFragment` | The one-field create dialog, opened with `data-modal-open="/fragment/room/new" data-modal-size="sm"`. |
| `GET /fragment/room/members?room={id}` | `Fragment` | `RoomMembersFragment` | The members panel. `room` must parse as a ULID and the caller must be owner or member, otherwise `w.WriteHeader(404)` and an empty body. |

The homepage nav: change `href="/host"` to `/rooms` and `href="/join"` to
`/rooms/join`. The word `join` is a DaisyUI component and the existing link
already leaks its two selectors into `app.css`; the selector diff after this
change tells you whether the new path removes them. If `/rooms/join` still
emits them, that is acceptable and known.

`GET /rooms/join` must be registered; Go's `ServeMux` prefers the literal
segment over `{id}`, so it does not shadow `GET /rooms/{id}`. Pin that in the
routes test.

### The join flow, in order

1. `ParseForm`. Read `code`, `NormalizeCode` it. If `!ValidCode`, re-render the
   form with "A room code is four letters or numbers." and 422. No query has run.
2. Read `character`. Empty means no character. Otherwise it must parse as a ULID
   and `GetCharacter` (owner-scoped) must find it, or 422 with "That character
   is not yours." The character is verified before the room lookup so a bad
   character cannot be used to probe codes without paying the rate limit.
3. `RoomJoinAttempts.Allow(sess.UserID.String(), time.Now())`. Refused means 429
   and the form re-rendered with "Too many attempts. Wait a minute and try
   again." A refused try still counts.
4. `GetOpenRoomByCode`. `sql.ErrNoRows` means 422 and "No open room has that
   code." `is_locked` means 422 and "That room is locked. Ask the GM to unlock
   it." A real error logs and `htmx.ServerError`.
5. `Store.JoinRoom`. Then `htmx.Toast("You joined <name>.")` and
   `htmx.Redirect` to `/rooms/{id}`.

The join page form posts with the same `hx-post`, `hx-target`, `hx-swap` and
`hx-status:422` trio the new-character form uses, so the typed code stays in
the field on failure. Pin the trio in a template test.

### Room page access

`RoomPage` and every room fragment use one helper in `room-page.go`:

```go
func (a *App) loadRoomMember(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, room.Role, bool)
```

It parses `{id}` (or `?room=` for fragments; pass which), runs `GetRoom`, and
decides:

- `owner_id == sess.UserID`: role GM. Closed rooms still open for the GM; the
  page shows the closed state with the Reopen button.
- `sess.RoomID != nil && *sess.RoomID == id`: role player. If the room is
  closed, call `Store.LeaveRoom`, `htmx.Toast("That room has closed.")` and
  redirect to `/rooms/join`. A page request uses `redirect`; a fragment answers
  `htmx.Redirect`.
- Otherwise: a page request redirects to `/rooms/join`; a fragment answers 404
  with an empty body.

`room.Role` is a string type with `RoleGM` and `RolePlayer`, defined in
`internal/room` now because phase 2's authority rules use the same type.

## Templates

Files, each `.templ` with a sibling `.go` holding its page-data types and every
helper. No comments in the `.templ` files. Reasoning goes in the `.go` file or
the handler.

- `pages/rooms.templ`, `pages/rooms.go`: `Rooms(data RoomsPageData)` page,
  `roomCard` partial, `NewRoomFragment()` and `const NewRoomPanel = "new-room"`.
  Copy the shape of `characters.templ`: `appBar`, `backLink(homeBack())`, an
  `h1`, a primary "New room" button on the right, then a scrolling section of
  cards with an empty state. A card shows name, the code or "Closed", a lock
  badge, and Enter / Reopen / Delete actions. Delete carries `hx-confirm`.
- `pages/room-join.templ`, `pages/room-join.go`: `JoinRoom(data JoinRoomPageData)`
  with the form and the character select. `Characters []queries.Character`
  comes from `GetCharacters`.
- `pages/room.templ`, `pages/room.go`: `Room(data RoomPageData)` page,
  `RoomMembersFragment(data RoomMembersData)`, `roomLockControl(data)` partial
  returned by the lock routes.

`RoomPageData` carries pre-formatted strings only: `ID`, `Name`, `Code`
(empty when closed), `Locked bool`, `Closed bool`, `Role room.Role`, and
`Members RoomMembersData`. The controller does every conversion.

### The shell layout

**Superseded 2026-09-07, after this shell was built and rejected.** What was
specified here -- a header carrying the code, the lock control and Close, over
a body split into a table region and a side panel holding the members list and
placeholders for initiative and chat -- is not what the room page is. See the
"Page composition" section of `plans/vtt-overview.md`, which now carries the
decision, and `server/templ/pages/room.go`, which carries the reasoning.

In brief: the room is one application window. A thin menu bar across the top
carries seven menus, the table fills everything under it, and a vertical
icon-only tool pill floats over the table's top-right corner. There is no side
panel, no members list on the page and no chat. Most menu items are disabled,
because the features behind them are later phases.

Two things from the original text survive and still matter:

```html
<room-page class="relative block h-dvh w-screen overflow-hidden">
<div id="tabletop" class="relative min-h-0 min-w-0 bg-base-300"></div>
```

**It is not `id="table"`.** `table` is a DaisyUI component and any bare
occurrence in a `.templ` file, including inside an attribute value, emits the
whole table family. The same applies to `list`, `status`, `tab`, `stack`,
`swap`, `menu` and `link` as bare words. Prefer `tabletop`, `room-lock`,
`room-code`.

## Homepage

Besides the two hrefs: when `session.RoomID` is non-nil, `Homepage` may show a
"Return to your table" link to `/rooms/{id}`. It needs no query; the room page
handles a closed room. Optional, but cheap and useful after a refresh.

## Tests

`server/routes_test.go`:

- Add every pattern above to `TestPanelRoutesMatchTheirOwnPatterns` or a new
  `TestRoomRoutesMatchTheirOwnPatterns` in the same shape, including
  `GET /rooms/join` resolving to its literal pattern and not `/rooms/{id}`,
  `POST /fragment/room/new` landing on `/fragment/`, and `GET /fragment/room/members`
  resolving to its pattern.
- `TestCrossSiteMutationsAreRefused` gains `POST /rooms`, `POST /rooms/join`,
  `DELETE /rooms/x`.

`server/internal/controllers/rooms_test.go`, `room-join_test.go`,
`room-page_test.go`, using `recordingDB`, `newPanelApp`, `panelPost` and
`oneRowDB` as the character tests do:

- Create: empty name is 422 with no statement run; a 200-rune name is 422; a
  valid name runs `CreateRoom` with a 16-byte id, the session's user id as
  owner, and a code that passes `ValidCode`; the response carries `HX-Redirect`
  to `/rooms/{id}` and a toast trigger.
- Join: a three-character code is 422 with zero statements; a lowercase valid
  code is normalised; an eleventh attempt inside a minute is 429 before the
  room lookup; a locked room (via `oneRowDB` returning `is_locked = 1`) is 422
  with the locked message; success runs `SetSessionRoom` keyed on the hash and
  answers `HX-Redirect`.
- Close: runs `CloseRoom` then `ClearRoomSessions` in that order, both bound to
  the room id.
- Page access: owner reaches the page; a member (session `RoomID` set) reaches
  the page with role player; a non-member is redirected to `/rooms/join`; a
  member of a closed room gets `ClearSessionRoom` and a redirect.
- Members fragment: a non-ULID `room` is 404 with an empty body and zero
  statements.

`server/internal/room/code_test.go`: `NewCode` returns four characters from
the alphabet across a thousand draws; `ValidCode` rejects `I`, `L`, `O`, `0`,
`1`, length 3, length 5, and lowercase; `NormalizeCode` uppercases and trims.

`server/internal/session/session_test.go`: `JoinRoom` runs `SetSessionRoom`
with the hash and mutates the struct; `LeaveRoom` runs `ClearSessionRoom`.

`server/templ/pages/rooms_test.go`: the new-room fragment pins its
`hx-post`, `hx-target`, `hx-swap`, `hx-status:422` trio against the block id;
the join form pins the same; the room page renders the GM controls for
`RoleGM` and not for `RolePlayer`.

## Verification

1. `make templ && make sqlc && make check`.
2. The CSS diff from `CLAUDE.md`, before and after `make css`. Every added
   selector must be one you can point at in the new markup. Note whether
   `.join` left.
3. Chromium walkthrough with two browser profiles: GM creates a room, copies
   the code; player joins with the code and a character, both see each other in
   the members panel after a refresh; GM locks, player's second profile is
   refused with the locked message; GM closes, player's next request lands on
   the join page with the toast; GM reopens and gets a new code; GM deletes from
   the list behind the confirm modal.

## Out of scope

The hub, the socket, the canvas, live members, the TypeScript build, renaming a
room from the page (the query exists; wire it when the header gets an edit
control), co-GMs, spectators as a distinct role, kicking (phase 3, because it
needs the socket closed too), and any room state beyond what the row holds.
