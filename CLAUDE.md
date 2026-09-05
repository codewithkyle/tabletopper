# Tabletopper

## What this repo is

`./server` **is the project.** Go + templ + htmx + DaisyUI, server-rendered. Every change
goes here. Migrations live in `./db/migrations`; the Docker image copies `./server` and
nothing else.

`./client` **is the old TypeScript SPA, kept only as a reference until the rewrite is
finished, then deleted.** It is not built, not served, and not part of the app. Read it to
see how a feature used to behave. Do not edit it. Do not import from it. Do not carry its
patterns, its class names, or its client-side state into `./server`.

The rules below are not suggestions. Follow them exactly. If a change cannot be made
without breaking one, stop and say so.

## Code comments

**Never reference a plan, rewrite, review, patch, revision, or notes markdown document from
a code comment, a Makefile, or any other file that ships.** No `PLAN.md`, no `REVIEW.md`,
no `NOTES.md`. Those documents are temporary and are deleted the moment the work they
describe is finished.

Write what the comment needs into the comment. If a command belongs in a comment, inline
the command. A comment that cannot stand on its own does not belong in the code.

## HTMX fragment routes

**Every route that returns partial HTML MUST be mounted under `/fragment/`.**

- A `/fragment/` route is a `GET` that returns partial HTML. Nothing else.
- Register it in `server/routes.go` behind `auth.Fragment(...)`, never `auth.RequireSession(...)`.
- Mutations keep their resource URLs. `POST /fragment/...`, `DELETE /fragment/...` and
  every other verb are forbidden — the subtree catch-all answers them with a 404.
- Never mount a full-page route under `/fragment/`. Never return partial HTML from a
  route outside it.
- Handlers set `Content-Type: text/html; charset=utf-8` and render the same templ
  component the full page render uses. A fragment is never a second copy of markup.
- Validate every query parameter against a known set. A bad value is
  `w.WriteHeader(http.StatusNotFound)` and an empty body. Never `http.NotFound` —
  it writes a page-shaped body.
- `middleware.Fragment` already sets `Cache-Control: no-store` and `X-Robots-Tag: noindex`.
  Do not set them per handler.

Call sites:

```go
// server/routes.go
mux.HandleFunc("GET /fragment/character/spell-card", auth.Fragment(app.SpellCardFragment))
```

```html
<!-- templ -->
<button hx-get={ "/fragment/character/info-row?field=" + field } hx-target="..." hx-swap="beforeend">
```

## Modals

**There are three modals. They are all we need. Do not add a fourth. Do not add a mode,
variant, or size option to an existing one. Never use `window.alert` or `window.confirm`.**

All three are `<dialog>` elements rendered once in `server/templ/layouts/base.templ` and
opened with `showModal()`. Clicking outside never dismisses them; every dialog ships its
own close control.

Buttons inside any dialog — including inside a fragment loaded into the content modal —
are solid `btn`. No `btn-ghost`, no `btn-soft`. Neutral buttons carry `border-base-content/50`.

### 1. Alert — tell the user something went wrong

`#alert-modal`, driven by `internal/htmx`. Handlers never touch the header directly.

```go
htmx.Error(w, "Heading", "Message.", http.StatusBadRequest)
htmx.ServerError(w)          // generic 500
htmx.NotFound(w, "character") // "That character no longer exists..."
```

The response body stays empty; the `noSwap` config in `base.templ` leaves the caller's
target untouched for every 4xx and 5xx.

### 2. Confirm — gate a destructive action

`#confirm-modal`, driven by `hx-confirm`. Put `hx-confirm` on the element that makes the
request. Nothing else is required.

```html
<button
  hx-confirm={ "You are about to delete " + character.Name + ". This action cannot be undone." }
  hx-delete={ "/characters/" + character.ID.String() }
  hx-target="closest character-card"
  hx-swap="delete"
  class="btn btn-soft btn-error"
>Delete</button>
```

Optional overrides, as data attributes on the same element:

| Attribute | Default |
| --- | --- |
| `data-confirm-heading` | `Are you sure?` |
| `data-confirm-label` | `Confirm` |
| `data-cancel-label` | `Close` |

Escape and the cancel button both drop the request.

### 3. Content — custom content and forms

`#content-modal`. Fire `modal:open` with the URL of a fragment:

```js
window.dispatchEvent(new CustomEvent("modal:open", {
    detail: { url: "/fragment/character/new", size: "lg" },
}));
```

Or from markup, with `hx-on:`:

```html
<button hx-on:click="window.dispatchEvent(new CustomEvent('modal:open', { detail: { url: '/fragment/character/new' } }))">
```

- `url` is required and MUST start with `/fragment/`. Anything else is refused and the
  dialog stays shut.
- `size` is optional: `sm` 24rem, `md` 32rem (default), `lg` 42rem, `xl` 56rem.
  These four are the whole set.
- The modal fetches with `GET` and swaps the response into its body. Loading, error and
  retry states are handled by the shell. The shell also supplies the ✕.
- The fragment brings its own heading and its own actions, as ordinary `hx-*` attributes.
  They are processed on arrival — never call `htmx.process()`.
- A form inside the modal posts to its normal resource URL. On failure, re-render the form
  with its errors and say nothing else; the modal stays open. On success, dismiss it with
  an `HX-Trigger` of `{"modal:close": true}` — add a helper to `internal/htmx` to send it,
  never `w.Header().Set("HX-Trigger", ...)` by hand, which clobbers a queued toast.

### Editing modal JavaScript

`server/public/js` is not a Tailwind `@source`. A class name written in a script is never
emitted. Render every class in templ and toggle `[hidden]` or an inline style from JS.
