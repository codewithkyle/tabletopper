# Player Journals

Per-character journals: a new Journal tab listing entries by title, created and
updated date, and a WYSIWYG editor that writes markdown.

This document is temporary. Delete it when the work lands. **Nothing that ships
may reference it** — not a code comment, not the Makefile, not `.dockerignore`.
Reasoning that needs to survive goes into the Go that renders or reads the
thing, per CLAUDE.md.

---

## 1. Decisions, and the evidence

Everything below was measured in a scratch build, not assumed.

### 1.1 The markdown lives in MySQL, not R2

R2 is right for maps and avatars and wrong for this. The difference is what we
do with the bytes: images are megabytes, served straight to the browser, never
queried. Journal entries are kilobytes, rendered into a page we are already
generating, and will eventually be searched.

Storing them in R2 costs:

- **A second write with no transaction.** The list needs title, `created_at`
  and `updated_at`, so there is a `journals` row either way. Every save becomes
  DB-write + R2-write, and the failure mode is a row whose `updated_at` moved
  while the object did not. `UploadMap`/`DeleteMapObjects` already carry that
  compensating-delete machinery; it earns its keep for multi-megabyte binaries
  and not for 4 KB of text.
- **A network round trip per entry.** Rendering a list of entries means N GETs
  at ~20–100 ms, in a request that is otherwise one query.
- **No search.** `FULLTEXT` on a `MEDIUMTEXT` column is available whenever
  search is built. R2 cannot.
- **Class A operations on every autosave.** A twenty-minute writing session is
  dozens of billable writes per entry.

Size is not the argument for R2 either way: 5,000 entries at 4 KB is 20 MB.
`inventory.description` is the existing precedent.

### 1.2 Markdown is the stored format; goldmark will render it, later

Portable, exportable, greppable, `FULLTEXT`-able, and safe to render — goldmark
emits no raw HTML unless `html.WithUnsafe()` is switched on, so stored text can
never inject a script tag. No sanitizer, no permanent attack surface.

**Nothing in this work renders markdown to HTML.** Phase 1 shows an entry only
inside the editor, so a server-side renderer would ship with no caller. The
read-only view is a follow-up (§9), and the evidence below is recorded here so
it does not have to be measured twice.

Measured against goldmark v1.8.6 (current latest) with default options:

```
js link      <p><a href="">click me</a></p>
data link    <p><a href="">x</a></p>
vbscript     <p><a href="">x</a></p>
JaVaScRiPt   <p><a href="">x</a></p>
raw html     <!-- raw HTML omitted -->
```

Dangerous protocols are neutralised case-insensitively and raw HTML is dropped.
**No link-sanitising hook is needed.** This matters because Tiptap *does* keep
`javascript:` in the markdown it serialises, even with its `protocols` option
set — that option governs autolink detection, not link sanitisation. The safety
lives entirely in goldmark at render time, which is the correct place for it:
the server is the only thing between the database and a browser.

A shared package-level `goldmark.Markdown` was race-tested with 64 concurrent
renders under `-race` and is clean, so one instance is built at init and reused.

Two things the follow-up must carry, found in review:

- goldmark's defaults do not render `~~strike~~`. StarterKit keeps strike in
  the schema (§1.4), so the renderer needs `extension.Strikethrough`.
- Check how `@tiptap/markdown` serialises a hard break. If it emits `<br>`,
  goldmark drops it as raw HTML and every Shift-Enter vanishes in the read view.

Because the editor is the only writer, stored markdown is always Tiptap's
normalised output — a narrow, predictable subset. goldmark never has to cope
with arbitrary hand-written markdown.

### 1.3 The editor is Tiptap, and it needs npm + esbuild

Tiptap shipped a first-party markdown extension in October 2025.
`@tiptap/markdown` 3.31.3, MIT, one dependency (`marked`), peers pinned to
`@tiptap/core` and `@tiptap/pm` 3.31.3. Bidirectional and CommonMark-compliant.

Round-trip, with a realistic entry using every construct we want — h2–h4, bold,
italic, ordered and unordered lists, a multi-paragraph blockquote, links:
**byte-identical**, with one normalisation (a blank line inserted between two
adjacent headings).

Bundle, esbuild, minified:

| build | raw | brotli |
| --- | --- | --- |
| StarterKit + Markdown | 434 KB | 117 KB |
| trimmed to exactly our feature list | 423 KB | 115 KB |
| *(htmx.min.js today)* | 35 KB | 11 KB |

Trimming saves ~2 KB brotli. The weight is `prosemirror-view` (23%),
`@tiptap/core` (19%), `prosemirror-model` (10%), `marked` (10%) — not the
extensions.

**Why the toolchain is justified here specifically:** the no-bundler
alternative (Toast UI, single UMD file) is ~400 KB of JavaScript *as well*,
plus its own theme CSS fighting DaisyUI. We do not save the payload by avoiding
npm; we only lose control of the editor's schema. And the schema is the data
contract — see below. The bundler buys authority over what survives a save.

### 1.4 The schema is wider than the toolbar

Constructs outside the editor's schema are **silently destroyed on save**, and
with autosave, permanently, seconds after a paste. Measured:

````
CHANGED  table          "| Monster | AC |..."   →  "| Monster | AC | | --- | --- | | Hag | 17 |"
CHANGED  image          "![map](.../marsh.png)" →  "map"
CHANGED  code block     "```\nfireball\n```"     →  "\`\`\` fireball \`\`\`"
CHANGED  strikethrough  "~~dead~~ alive"        →  "dead alive"
CHANGED  horizontal rule "above\n\n---\n\nbelow" →  "above\n\nbelow"
CHANGED  task list      "- [ ] ask the DM"      →  "- ask the DM"
````

So: **ship StarterKit's full schema, expose a seven-button toolbar.** StarterKit
already models code, code blocks, strike, horizontal rules, hard breaks and
every heading level, so pasted content survives and round-trips even though
there is no button for it. That costs 2 KB brotli, which is the whole reason
not to trim.

The same reasoning settles the heading question. The toolbar offers paragraph
and h2–h4 only, so markdown the editor writes never contains a `#` heading and
the entry title stays the page's one `h1`. A pasted `# Heading` is still
modelled, still saved, and styled by size like the rest.

Tables and images remain outside StarterKit and are out of scope — see §9.

---

## 2. Storage

Two migrations. Today's last is `20260905210200`, so these follow it.

### 2.1 `db/migrations/20260905220000_create_journals_table.sql`

Mirrors `inventory`: owner denormalised so a single-row write filters on id,
character and owner with no join, and no `position` column because ULIDs sort
lexicographically by creation time.

```sql
-- migrate:up
CREATE TABLE IF NOT EXISTS journals (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    title VARCHAR(255) NOT NULL DEFAULT '',
    body MEDIUMTEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_journals_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS journals;
```

Notes that belong in the migration's own comment block when it is written:

- `MEDIUMTEXT`, not `TEXT`. MySQL runs strict, so a 64 KB overflow returns a
  driver error and reaches the user as a 500 on a field they were entitled to
  overfill — the same reasoning as `inventoryDescriptionLimit`. The app-level
  cap (§2.3) produces the friendly message instead.
- Every column has a default, `body` included, so the insert names three
  columns and creating an entry cannot carry entry data.
- **No `FULLTEXT` index yet.** InnoDB maintains one on every `UPDATE` of the
  indexed column, and the editor updates `body` on a one-second debounce for as
  long as someone is writing. Adding the index to a 20 MB table when search is
  built takes seconds; paying for it on every keystroke pause until then does
  not.
- `ON UPDATE CURRENT_TIMESTAMP` only fires when a column actually changes, so a
  debounce that re-posts identical text leaves `updated_at` alone and the date
  on the list is honest.

### 2.2 `db/migrations/20260905220100_drop_character_notes.sql`

`characters.notes` is written as `''` by `CreateCharacterFromName` and read by
nothing; `unownedColumns` in `character-panels_test.go` says so in as many
words. Journals supersede it.

```sql
-- migrate:up
ALTER TABLE characters DROP COLUMN notes;

-- migrate:down
ALTER TABLE characters ADD COLUMN notes TEXT NOT NULL;
```

Consequences: remove `notes` from the insert in `server/sql/characters.sql`,
remove it from `unownedColumns` and from the two `?field=notes` cases in
`character-panels_test.go`, and let `make sqlc` drop the field.

### 2.3 Limits

Enforced in the controller, in the units the column counts:

| const | value | unit | why |
| --- | --- | --- | --- |
| `journalTitleLimit` | 255 | runes | `VARCHAR(255)` counts characters |
| `journalBodyLimit` | 262,144 | bytes | see below |

256 KB is roughly forty pages of prose. `MEDIUMTEXT` takes 16 MB and Go's
`ParseForm` takes 10 MB, so neither is the ceiling; the cap is there because
the whole body is posted on every debounce and a runaway paste should be
refused with a message, not shipped every second.

### 2.4 sqlc

`server/sql/journals.sql`, every query scoped to the owner so a handler never
checks ownership after the fact:

| query | shape | notes |
| --- | --- | --- |
| `ListCharacterJournals` | `:many` | **`SELECT id, title, created_at, updated_at` — never `body`.** `ORDER BY id DESC`, newest first; the index serves it. |
| `GetJournalEntry` | `:one` | includes `body`; the editor page only |
| `InsertJournalEntry` | `:execresult` | **`INSERT ... SELECT` from `characters`**, exactly as `InsertInventoryItem`: id from the handler, owner and character off the characters row |
| `UpdateJournalEntry` | `:execresult` | title + body, filtered on all three ids |
| `DeleteJournalEntry` | `:execresult` | filtered on all three ids |

`:execresult` on all three mutations, not `:exec`. With `ClientFoundRows` on
(see `database.Open`), zero matched rows means the row is not this user's, and
the handler answers 404 rather than pretending to save. The insert is the same
shape for the same reason: a plain `VALUES` insert would take `character_id`
straight from the URL and hang entries off a stranger's sheet, visible only to
the sender. Selecting off `characters` makes a character that is not this
user's insert nothing.

The list query omitting `body` is load-bearing: 200 entries at 4 KB is 800 KB
shipped to render a list of titles.

**`sqlc.yaml` needs a new override.** First match wins and `nullable: true` does
not narrow it, so a `NOT NULL` column named `character_id` picks up the
`*ulid.ULID` meant for nullable ones unless it is named first. Add alongside the
existing three:

```yaml
- column: "journals.character_id"
  go_type: *ulid
```

`id` and `owner_id` are already covered by the `*.id` and `*.owner_id`
wildcards. Regenerate with `make sqlc` after `make db` (schema.sql is
gitignored and produced by dbmate).

---

## 3. Rendering

Deferred to the follow-up in §9. The evidence and the two open items are in
§1.2. No `internal/markdown` package, no goldmark dependency, no renderer test
in this work.

---

## 4. Build pipeline

This follows the existing precedent exactly: **the tool is gitignored, the
output is committed.** `build/bin/tailwindcss` is ignored and
`server/public/css/app.css` is checked in; `*_templ.go` and the sqlc package
are checked in too. Same here.

Consequences worth stating: the Docker image copies `./server` and
`./server/public`, both of which already contain everything they need. The
deploy is untouched. CI does not need Node for the server (though the existing
workflow is stale and still builds the legacy `client` and `wss` — see §8).

### 4.1 Layout

| path | role |
| --- | --- |
| `package.json`, `package-lock.json` | repo root, beside `Makefile` and `sqlc.yaml`; lockfile committed |
| `node_modules/` | **added to `.gitignore`** |
| `server/js/journal-editor.js` | source; mirrors `server/css/` living outside `public/` |
| `server/public/static/journal-editor.js` | build output, committed |

The output goes in `static/`, not `js/`, to keep a crisp invariant:
**`public/js/` is served exactly as authored and is never built; `public/static/`
is not authored here.** `htmx.min.js` already establishes that. Mixing a build
output into `public/js/` alongside nine hand-written modules invites someone to
hand-edit a generated file.

`package.json` lives at the root and not in `server/` for a reason the Docker
build settles: the compose build context is `.` and the Dockerfile copies
`./server/.` wholesale, so a `server/node_modules` would land in the builder
stage. See §4.4.

### 4.2 Dependencies

```
@tiptap/core  @tiptap/pm  @tiptap/starter-kit  @tiptap/markdown   (all MIT, 3.31.3)
esbuild                                                           (devDependency)
```

~23 MB of `node_modules` on dev machines. Node 24 locally (24.16 measured);
pin `engines` and say so in the Makefile comment.

### 4.3 Makefile

New `js` target, added to `.PHONY` and to `run`'s dependency list beside
`templ`, `sqlc` and `css`:

```make
js:
	./node_modules/.bin/esbuild ./server/js/journal-editor.js \
		--bundle --minify --format=esm --target=es2022 \
		--outfile=./server/public/static/journal-editor.js
```

Unlike `css`, this one **is** minified: it is vendored third-party code nobody
reads in a diff, so the readable-diff argument that keeps `app.css` unminified
does not apply. Brotli at the edge still does the compression that matters.

The Makefile comment must inline the `npm ci` instruction rather than pointing
at this document.

### 4.4 `.dockerignore` and the Dockerfile

There is no `.dockerignore`, and `compose.yaml` builds with `context: .`. Every
`make run` today ships the repo root to the daemon, including the 237 MB
`client` tree. Adding `node_modules` on top is what makes it worth fixing now.

New `.dockerignore` at the repo root:

```
.git
client
node_modules
build
db
server/css
server/js
**/.env
```

`server/css` and `server/js` are build inputs whose outputs are committed under
`server/public`; the Go build reads neither. `server/public/css` is a different
path and stays in. `.env` files never belong in a build context even when no
stage copies them into the final image.

The Dockerfile keeps its two `COPY` lines and gains a comment stating the
invariant that makes them sufficient: the image never runs npm, esbuild,
Tailwind, templ or sqlc, because every one of their outputs is committed. The
comment stands on its own — no pointer to this file.

---

## 5. The editor

`server/js/journal-editor.js`, a small module that binds to the editor page.

### 5.1 Configuration

```js
const field = form.querySelector('textarea[name="body"]');

new Editor({
  element: mount,
  extensions: [
    StarterKit.configure({ link: { openOnClick: false } }),
    Markdown,
  ],
  content: field.value,
  contentType: "markdown",
});
```

Full StarterKit for the reasons in §1.4.

**The initial content is read from the hidden textarea, and only from there.**
templ escapes the textarea's text, so the stored markdown reaches the browser
as data. Inlining it into a `<script>` block instead would make every entry a
script-injection vector.

### 5.2 Autosave — reuse the existing pattern, do not invent one

Tiptap is not a form control, so htmx's `input` trigger will not see it. Bridge
it: the page carries a hidden `<textarea name="body">` inside the form, and
Tiptap's `onUpdate` writes to it and dispatches a bubbling `input` event.

```js
onUpdate: () => {
  field.value = editor.getMarkdown();
  field.dispatchEvent(new Event("input", { bubbles: true }));
}
```

htmx's existing `input delay:1s` debounce on the form then behaves exactly as
it does for every other panel — same trigger, same 422 handling, same error
block. No new save mechanism, no new JavaScript save path.

**A successful save does not toast.** `finishInventoryRow` toasts "X saved."
because an inventory field is a few words and a save is an event. A journal
save is a pause between sentences, and `toast.js` stacks its toasts for five
seconds each. The save handler renders the error block empty and sets no
header; delete keeps its toast. That is a `finishJournalEntry` beside
`finishInventoryRow`, differing in the one line.

### 5.3 Toolbar

Seven controls: bold, italic, link, blockquote, heading (a `<select>` for
paragraph + h2–h4, per §1.4), bullet list, ordered list. Plus Ctrl/Cmd-B, -I,
-K.

Every button is `type="button"`. The toolbar sits inside the autosaving
`<form>`, and a bare `<button>` there is a submit. The heading `<select>` is
posted as a field with the rest of the form; the handler ignores it.

**Every class name is rendered in templ and toggled from JS via `[hidden]` or an
attribute.** `server/public/js` is not a Tailwind `@source`, so a class written
in a script is never emitted. The active state of a toolbar button is an
`aria-pressed` attribute styled from `journal.css`; it is accessible and it
sidesteps the problem entirely.

### 5.4 The link dialog

Link insertion uses the content modal — **never `window.prompt`**, which is as
banned as `window.confirm`. There is no server resource behind it, so this is
the one place the modal carries a form that does not post. The design:

- `GET /fragment/character/journal-link` renders a static fragment: a heading,
  one `<input type="url" name="href">`, and `Close` / `Insert link`. The form
  carries a `data-journal-link` attribute and no `hx-*` on the form itself.
  It takes no query parameters; any query string is a 404 with an empty body.
- The toolbar button and Ctrl/Cmd-K dispatch `modal:open` with that URL. Before
  dispatching, the editor records the href under the cursor, if any.
- The editor listens on `#content-modal` for htmx's after-swap event (the same
  family `content-modal.js` already listens to; confirm the exact name against
  the vendored htmx). When the swapped content contains `[data-journal-link]`,
  it fills the field with the recorded href and attaches a `submit` handler.
- That handler calls `preventDefault`, reads the field, and runs
  `extendMarkRange('link').setLink({ href })` — or `unsetLink()` when the field
  is empty, which is how a link is removed — then dispatches `modal:close`.

No JavaScript in the templ attributes; the one hook does both the prefill and
the submit. The fragment is static, so it has no page-data type; its reasoning
goes on the handler.

---

## 6. Routes and pages

### 6.1 Routes

Added to `server/routes.go`.

```go
mux.HandleFunc("GET /characters/{id}/edit/journal", auth.RequireSession(app.CharacterJournalPage))
mux.HandleFunc("GET /characters/{id}/edit/journal/{entryId}", auth.RequireSession(app.CharacterJournalEntryPage))
mux.HandleFunc("POST /characters/{id}/journal", auth.RequireSession(app.CreateJournalEntry))
mux.HandleFunc("POST /characters/{id}/journal/{entryId}", auth.RequireSession(app.SaveJournalEntry))
mux.HandleFunc("DELETE /characters/{id}/journal/{entryId}", auth.RequireSession(app.DeleteJournalEntry))

mux.HandleFunc("GET /fragment/character/journal-link", auth.Fragment(app.JournalLinkFragment))
```

- Pages under `/edit/`, matching the other tabs. Mutations keep resource URLs
  off `/edit/`, matching inventory and spells.
- **Create is a plain form post, not htmx.** A `<form method="post">` with one
  button; the handler inserts a blank entry and 303s into its editor. The
  browser follows it. This is simpler than the character-creation dialog
  because there is no field to collect — the title is edited in place.
- **Delete answers 200, not 204.** `noSwap` lists 204 and would override
  `hx-swap="delete"`, leaving a deleted card on screen. Same trap as inventory.
- **Pages redirect; mutations answer `htmx.NotFound`.** `htmx.NotFound` writes
  an `HX-Trigger` header and an empty 404 body, which is right for an htmx
  request and a blank screen for a browser navigation. So:
  - The two `GET` pages parse `{entryId}` with `ulid.Parse` and, on a failure
    or a `sql.ErrNoRows` from `GetJournalEntry`, redirect to
    `/characters/{id}/edit/journal` — the same shape as `loadCharacter`
    redirecting to `/characters`.
  - The create handler is a browser post too. A character that is not this
    user's inserts nothing, and it redirects to `/characters`.
  - Save and delete are htmx. A bad id or zero matched rows is
    `htmx.NotFound(w, "journal entry")`, matching `inventoryItemID`.

### 6.2 The tab

`server/templ/pages/character-tabs.templ` gains a fourth link and its const
block a fourth name:

```go
characterTabJournal = "journal"
```

### 6.3 Pages

| file | contents |
| --- | --- |
| `server/templ/pages/journal.go` | `JournalPageData`, `JournalEntry`, `JournalEntryPageData`, `Timestamp` — every field a string, the controller does every conversion, per `InventoryItem` |
| `server/templ/pages/journal.templ` | the list: one card per entry, title (or "Untitled entry"), created and updated, delete button with `hx-confirm`, and the one-button create form |
| `server/templ/pages/journal-entry.templ` | the editor: title input + hidden body textarea inside one `savingPanel`, plus the toolbar and mount point |
| `server/templ/pages/journal-link.templ` | the link dialog fragment (§5.4) |
| `server/internal/controllers/journal.go` | six handlers |

`savingPanel` is reused as-is — the journal entry form is a panel like any
other, and gets `PanelFormErrors` and the `hx-status:422` override for free.
It renders a fixed uppercase heading above its children, which the entry page
will carry above the title input; "Entry" is the label.

**Timestamps are localised in the browser, with the platform's own API.**
`compose.yaml` runs MySQL at `+00:00` and the DSN has `parseTime=true`, so the
controller holds a UTC `time.Time` and knows nothing about the reader's zone.
Only the browser does. The markup for each date is:

```html
<local-time><time datetime="2026-09-05T18:04:11Z">5 Sep 2026, 18:04 UTC</time></local-time>
```

- `datetime` is RFC 3339 in UTC — the machine-readable value, and the input to
  the upgrade.
- The text inside is the server's UTC rendering. It is what shows before the
  module runs, with JavaScript off, and in a test.
- `server/public/js/local-time.js` defines the `<local-time>` element. Its
  `connectedCallback` parses the child's `datetime`, sets its text with
  `Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" })`
  and its `title` with the `full`/`long` styles, which carry the zone name.
  `undefined` takes the browser's locale, so a reader in Sydney sees Sydney
  time in Australian order and nobody ships a locale file. A custom element
  rather than a query at load so a card swapped in later upgrades itself, and
  because `soft-loading` and `toaster-component` are the house style.
- It is a few hundred bytes with no dependency, hand-written and served as
  authored from `public/js/`, so it goes in `base.templ` beside the other
  modules rather than through the `Base` script slot.

`JournalEntry` carries each date as a `Timestamp{ISO, Text string}`, both
produced by the controller, so the template does no formatting — the
`InventoryItem` rule.

Why not dayjs: its `format()` is not locale-aware without the
`localizedFormat` plugin and a locale file per language, and its `timezone`
plugin does its conversion by calling `Intl.DateTimeFormat` — the API above,
wrapped. It would also be a second npm dependency needing either its own
esbuild entry or a hand-vendored copy in `public/static/`. The platform call
is the whole job here.

The reasoning for the server half lives in `journal.go`; the client half in
the module's own comment.

**No comments in any `.templ` file.** Reasoning goes in `journal.go` (the
page-data types) or the controller. This is the rule with the silent failure
mode: a bare lowercase word in a `.templ` file is a Tailwind class candidate.

### 6.4 The base layout needs a script slot

`layouts.Base(title string)` loads every script globally. 117 KB of editor has
no business on the characters list. Change the signature to:

```go
templ Base(title string, scripts ...string)
```

Verified against templ v0.3.977 in a scratch module: a variadic component
parameter generates and builds, and every one of the eleven existing
`@layouts.Base("...")` call sites keeps working unchanged. The journal entry
page becomes:

```go
@layouts.Base("Journal | Tabletopper", "/static/journal-editor.js")
```

Rendered as `<script type="module">`, which is what an esbuild `--format=esm`
bundle needs. The list page loads none of it.

---

## 7. Styling

The editor's content area is DOM that ProseMirror builds, not markup from a
`.templ` file, so **Tailwind emits nothing for it** — and
`@tailwindcss/typography` is npm-only and would need vendoring like DaisyUI
was.

Hand-write `server/public/css/journal.css`, scoped to the mount container,
native nesting, served as authored — exactly like `core.css`, `toast.css` and
`soft-loading.css`. Around 40 lines covering h1–h6, `p`, `ul`/`ol`,
`blockquote`, `a`, `strong`/`em`, `code`/`pre`. Headings are styled by size;
h1 is included because a pasted one survives (§1.4). It also carries the
toolbar's `[aria-pressed="true"]` state from §5.3. The read view in §9 reuses
the file by sharing the container class.

Add the `<link>` to `base.templ`.

After touching anything under `server/templ`, run the selector diff:

```sh
cp server/public/css/app.css /tmp/app.before.css && make css
diff <(grep -oE '^\s*\.[^ {,:]+' /tmp/app.before.css | sort -u) \
     <(grep -oE '^\s*\.[^ {,:]+' server/public/css/app.css | sort -u)
```

Every added selector should be one we can point at in the markup just written.

---

## 8. Tests

New:

- `server/internal/controllers/journal_test.go` — mirroring `inventory_test.go`:
  unparseable entry id answers 404 on save and delete and redirects on the
  page; no matching row answers 404; create for a character that is not this
  user's inserts nothing and redirects; validation fails before the write; an
  oversized title or body is rejected with a message rather than a 500.
- `server/templ/pages/pages_test.go` — the entry page's form carries
  `hx-trigger="input delay:1s"` and the 422 override, pinned the way the
  inventory row's are; the link fragment renders `Close` before `Insert link`;
  the list renders each date as a `<time datetime>` inside `<local-time>`,
  with the RFC 3339 value in the attribute and the UTC text as its content.

Existing, **will break and must be updated**:

- `assertCharacterTabs` in `server/templ/pages/pages_test.go` iterates a fixed
  list of hrefs. It needs `base + "/edit/journal"` added, or every editor page
  test fails.
- `TestPagesRenderConcurrently` — add the list page, the entry page and the
  link fragment.
- `unownedColumns` and the `?field=notes` cases in `character-panels_test.go`
  lose `notes` with the column (§2.2).

The stale `.github/workflows/deploy-docker.yaml` still installs Node 18 and runs
`npm ci` in `client` and `wss`, neither of which is part of this app any more.
It needs attention regardless of this feature; adding npm to the server build is
a good moment to fix it, but it is **not** a blocker and not part of this work.

---

## 9. Out of scope

- **Rich read view.** Phase 1 renders entries in the editor only. The
  follow-up is `server/internal/markdown` with one exported
  `Render(src string) (templ.Component, error)` over a package-level
  `goldmark.New()` — defaults plus `extension.Strikethrough`, **never
  `html.WithUnsafe()`** — returning `templ.Raw` over the output, which is safe
  precisely because goldmark produced it. Its test pins the four hostile inputs
  from §1.2, so a future goldmark that stops neutering `javascript:` fails a
  test rather than a user. The hard-break question in §1.2 is answered first.
- **Search.** With the read view or after it; the `FULLTEXT` index is added
  then, not before (§2.1).
- **Images.** Someone will paste a map screenshot. That is an upload to R2 —
  which is what R2 is genuinely for — plus an `assets` row with a new `type`,
  plus an editor extension. A real second phase, not a stretch goal here.
- **Tables.** Outside StarterKit; pasted tables are mangled (§1.4). Acceptable
  for v1, worth a note in the UI if it turns out to bite.
- **Sharing a journal with the party / DM.** Journals are private to the owner;
  every query is owner-scoped. Note that this is also what makes Tiptap's
  retained `javascript:` hrefs (§1.2) self-XSS at worst until then; sharing
  goes through the goldmark read view, never through the editor.

---

## 10. Decisions made

1. **`characters.notes` is dropped** in this work (§2.2). Written as `''`,
   read by nothing, and journals are what it was for.
2. **Heading levels:** h1 is removed from the toolbar's select, not shifted at
   render time (§1.4). The title is the page's h1; pasted h1s survive and are
   styled by size (§7).
3. **New entries are blank** and titled in place, with an "Untitled entry"
   placeholder on the list (§6.1, §6.3).
4. **`package.json` at the repo root**, with a `.dockerignore` and a Dockerfile
   comment to go with it (§4.4).

---

## 11. Order of work

1. Both migrations + `make db` + `sqlc.yaml` override + `characters.sql` +
   `make sqlc`. (§2)
2. `Base` script slot + the fourth tab + `assertCharacterTabs`. (§6.2, §6.4, §8)
3. List page, create, delete, entry page, `local-time.js`. No editor yet — a
   visible `<textarea>` stands in for the mount. (§6)
4. `.dockerignore` + Dockerfile comment + npm + esbuild + Makefile target. (§4)
5. Tiptap editor + toolbar + the autosave bridge + the link fragment. (§5)
6. `journal.css` + the selector diff. (§7)
7. `make check`.

Steps 1–3 ship a working journal on their own. The editor is additive.
