# Front-End Migration: Brixi → Tailwind + DaisyUI

Replaces the Brixi CSS framework in `server/` with Tailwind 4 + DaisyUI 5.

**Scope:** Phases 1–4. Phase 5 (SSR-ing the lit-html components) is deferred — see the bottom.

**Applies to `server/` only.** `client/` is legacy and is not migrated.

---

## Ordering note

This reorders the original sketch. Two changes, both deliberate:

| Original | Here | Why |
|---|---|---|
| DaisyUI first, then Tailwind CLI | Tailwind CLI first, then DaisyUI | DaisyUI 5 is a Tailwind plugin (`@plugin "daisyui"` inside the Tailwind entry CSS). It cannot be installed before Tailwind exists. |
| Migrate classes, then build theme | Theme, then migrate classes | 44 of the 66 Brixi classes in use are colors and typography — exactly what the theme defines. Migrating first means writing them against stock Tailwind, then rewriting them once the theme lands. |

---

## Why this is worth doing

- Brixi is unmaintained; its author has moved to DaisyUI for new work.
- `brixi.css` is **420KB of the 521KB** of CSS currently served, for **66 classes** actually used.
- Rooms, monsters, spells, and the tabletop UI don't exist yet. Every page built on Brixi raises the price of this migration.
- Tailwind 4 ships a standalone binary, so this does **not** reintroduce the Node build we just removed from the server path.

---

## The trap that governs everything

**Brixi and Tailwind share class names with a 4× value difference.**

Brixi's scale is `1 unit = 1rem`. Tailwind's is `1 unit = 0.25rem`. So `mb-1`, `mb-3`, `p-1`, `p-2`, `px-1`, `px-2`, `mr-1`, `mr-2` all exist in **both** frameworks and mean different things.

A migration that "leaves the classes that already look right" silently renders them at a quarter size. It compiles, it renders, and nothing errors — the whole app just becomes cramped.

Two consequences:

1. **All 24 spacing/sizing classes must be converted, including the ones that look identical.** The mapping is a clean ×4 with no rounding (Appendix A), so it is scriptable.
2. **Do not run both frameworks at once.** Beyond the collision, `brixi.css` is compiled with `important: true` — **5,375 `!important` declarations across ~5,309 rules**. Tailwind cannot win a specificity fight with it. Phase 4 is therefore a single pass that deletes `brixi.css` in the same commit, not a page-by-page migration.

### Measured: which classes fail loudly, and which fail silently

Every Brixi class in use was fed to Tailwind 4.3.3 to see whether it generates a
rule. The 74 split cleanly, and the split is what makes Phase 4 checkable:

| | Count | What happens when `brixi.css` is deleted |
|---|---|---|
| **Vanishes** | 32 | Tailwind emits nothing. The style disappears. Visible on sight. |
| **Silent, same meaning** | 18 | Tailwind emits an equivalent rule. Safe to leave alone. |
| **Silent, wrong value** | 24 | Tailwind emits a rule at a **different value**. No error, no missing style. |

The 24 in the last row are the entire risk of this migration:

- 23 spacing classes render at **¼ size** (`mb-1`, `p-2`, `pt-1.75`, …).
- `max-w-1024` goes the other way: Brixi `1024px`, Tailwind `calc(var(--spacing) * 1024)` = **256rem = 4096px**, 4× too *large*.

The other 18 are genuinely equivalent in both frameworks and need no work:
`absolute` `bg-white` `block` `border-1` `border-solid` `border-t-1` `fixed`
`font-bold` `font-medium` `font-semibold` `h-full` `h-screen` `inline-block`
`max-w-full` `relative` `text-center` `w-full` `w-screen`

**The useful consequence:** `grep -c brixi` is not a sufficient Phase 4 check,
because the dangerous 24 leave no trace once `brixi.css` is gone. Grep for the
24 class names themselves. That list is Appendix A plus `max-w-1024`.

Incidental: `h-6` / `w-6` appear on SVG icons in `alerts.js`, `select.js` and
`spell-slots-table.js`, copied from a Heroicons snippet. Neither Brixi nor any
current stylesheet defines them, so today they do nothing. Once Tailwind is
linked they start sizing those icons to 1.5rem. Probably what was intended,
but it is a visual change arriving from a phase that touches no markup.

---

## Phase 1 — Tailwind CLI ✅ DONE

Standalone binary, no Node. Pinned to **v4.3.3** rather than `latest`, so the
build is reproducible:

```bash
mkdir -p build/bin
curl -sL -o build/bin/tailwindcss \
  https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.3/tailwindcss-linux-x64
chmod +x build/bin/tailwindcss
```

`build/bin/` is in the root `.gitignore` (~107MB binary).

**Entry stylesheet** at `server/css/app.css` — source lives outside `public/`,
build output goes into it:

```css
@import "tailwindcss" source(none);
@source "../templ/**/*.templ";
@source "../public/js/**/*.js";
```

**Makefile:** `css` added to the `run` chain, plus a `css-watch` target for
iteration.

### Two deviations from the original sketch

**1. `source(none)` instead of automatic detection.**

Tailwind 4 walks up to the git root and scans everything not gitignored. Here
that means `client/` — the legacy SPA. Measured: auto-detection emitted **137
selectors / 16.5KB** against **62 selectors / 10.2KB** for the explicit config,
and the 75 extras included pure garbage extracted from TypeScript type
annotations (`[key:string]`, `[level:string]`, `15s`, `5rem`). Every class the
explicit config generates is also in the auto set, so nothing is lost by
opting out.

**2. `public/js/**/*.js` is a source, not just `.templ`.**

The lit-html components build class strings client-side, so their classes never
appear in a template. This was missed by the original class survey, which only
scanned `.templ`. See the revised Phase 4 scope.

**Verified:** `make css` → 10,195 bytes containing `.w-full{width:100%}`. Both
globs proven live by classes unique to each — `justify-between` (templ-only)
and `h-6` (js-only) both appear in the output.

> The built `server/public/css/app.css` is **committed**, not gitignored. The
> only CI workflow in the repo still builds `Dockerfile.http` / `Dockerfile.wss`
> for the old client — files that no longer exist — so nothing automated builds
> `server/`. Until that is replaced, a checked-in stylesheet is the difference
> between a deploy that works and one that silently ships unstyled.

> `app.css` is **not yet linked** in `base.templ`. It gets linked in Phase 3,
> when there is a theme worth looking at.

---

## Phase 2 — DaisyUI ✅ DONE

DaisyUI **5.7.28** is wired in as a Tailwind plugin. `make css` now emits
**64,459 bytes** (up from 10,195), and prints `/*! 🌼 daisyUI 5.7.28 */`.

### Deviation: the plugin had to be vendored

The sketch above claimed "the standalone binary bundles Node, so `@plugin`
resolves without an `npm install`". That is wrong. The binary bundles Node's
**runtime**, not any npm **packages**:

```
Error: Can't resolve 'daisyui' in '/home/andrewk9/Personal/tabletopper/server/css'
```

So the plan's own fallback applies. The two prebuilt bundles from the DaisyUI
release (MIT) are committed to `server/css/vendor/`:

| File | Size | Used by |
|---|---|---|
| `daisyui.js` | 349 KB | `@plugin "./vendor/daisyui.js"` — active now |
| `daisyui-theme.js` | 47 KB | Phase 3 only — not yet referenced |

392 KB committed, versus reintroducing `node_modules` to a repo whose root has
none. This pins the version byte-for-byte and keeps `make css` working from a
fresh clone with only the Tailwind binary to fetch.

> **Phase 3 note:** the theme blocks must use `@plugin "./vendor/daisyui-theme.js"`,
> not `@plugin "daisyui/theme"`. Same resolution problem.

**Verified:** a scratch page using `btn btn-primary` and `card` emits `.btn`,
`.btn-primary`, `.card`, `.card-body`. The real build emits none of them,
confirming DaisyUI tree-shakes against the declared `@source` globs.

### Measured: what DaisyUI actually costs

| Source scanned | Output | Delta |
|---|---|---|
| Phase 1 baseline (no DaisyUI) | 10,195 | — |
| DaisyUI, empty source | 11,004 | +11,004 unconditional base |
| ...plus `menu` | 18,785 | **+7,781** |
| ...plus `label` | 12,014 | +1,010 |
| ...plus `link` | 11,287 | +283 |

DaisyUI adds **44 selectors** to the real build. Only **3** correspond to
classes this app already writes; the other 41 are their sub-components
(`menu-title`, `modal-box`, `tooltip-content`, …) pulled in transitively.


### ⚠️ Collisions — CORRECTED in Phase 3

The original count here was **3** (`menu`, `label`, `link`). That was wrong.
It was produced by intersecting DaisyUI's added selectors against class names
found in `.templ` files only. Two more classes never appear in a template: they
are emitted by the lit-html components in `public/js/`, and styled by the app's
own stylesheets.

Redone in Phase 3 by intersecting DaisyUI's 35 added selector-classes against
**both** the classes the app writes (132, from `.templ` + `public/js/`) **and**
the classes the app's own stylesheets style (787, from `public/css/*.css`):

| Class | Written by | Styled today by | Outcome |
|---|---|---|---|
| `menu` | `homepage.templ:21` | nothing | **deleted** — provably dead, 7.7KB |
| `input` | `js/input.js` | `css/input.css` | **excluded** — broke the sheet, see below |
| `select` | `js/select.js` | `css/select.css` | **excluded** |
| `label` | `skills-table.js`, `saving-throws-table.js` | those two stylesheets | **excluded** |
| `link` | `server-error.templ:21` | `css/link.css` | **excluded** |

`.input` was the damaging one. `input-component` renders a `<label>` above an
`<input>`; DaisyUI's `.input` sets `display:inline-flex`, which the app's own
CSS never overrides because it never sets `display` on that element. Result:
every label on the character sheet overlapped the field above it. Caught by
screenshotting the real page, not by reading CSS.

### Correction: cascade layers, not load order, decide these

Everything Tailwind and DaisyUI emit is inside `@layer` — measured on the real
build: `properties` (941B), `theme` (568B), `base` (9.7KB), `utilities` (50KB),
and nothing outside them. Unlayered declarations beat **every** layer regardless
of specificity or link order.

Two consequences the earlier analysis got wrong:

1. **Load order is not what protects the app's CSS.** Linking `app.css` first is
   still right (it reads as the base layer), but `link.css` would win from any
   position. Verified: `.link`'s color is unchanged with `app.css` linked ahead
   of it.
2. **A collision only bites on properties the app's CSS does not set.** That is
   exactly why `.input` hurt and `.label` did not — `skills-table.css` sets
   `color` on `.label` but not `display`.

The same rule defuses a hazard worth recording: DaisyUI's modal scroll-lock
emits `:root:not(span){overflow:var(--page-overflow)}`, specificity `0,1,1`
against `core.css`'s `html{overflow:hidden}` at `0,0,1`, and it resolves to
*invalid-at-computed-value-time* when no modal is open — which would unset
`overflow` on a fixed-viewport app. It is inert purely because it is layered.
Verified in a headless render: `html`'s computed `overflow` stays `hidden`.

## Phase 3 — Custom theme ✅ DONE

Two themes ported from the hand-built tokens in `public/css/core.css`, DaisyUI's
built-ins turned off, `app.css` linked in `base.templ`, and four component groups
excluded until Phase 5. `make css` now emits **38,844 bytes** (down from 64,459
at the end of Phase 2 — the exclusions and the dead `menu` more than pay for the
theme).

### What was done

1. **Deleted the dead `menu` class** from `homepage.templ:21`. One occurrence in
   the codebase, no stylesheet defined it, no JS queried it. −7.7KB.
2. **`themes: false`** on the DaisyUI plugin. It ships `light --default` and
   `dark --prefersdark` enabled, which are the same two slots the custom themes
   claim; leaving them on means two competing `:where(:root)` blocks.
3. **`exclude: input, select, label, link`** — see the corrected collision table
   in Phase 2. These come back in Phase 5.
4. **Two `@plugin "./vendor/daisyui-theme.js"` blocks**, `tabletopper`
   (`default`) and `tabletopper-dark` (`prefersdark`). The app has no theme
   toggle, and those two flags reproduce `core.css`'s pure
   `prefers-color-scheme` behavior exactly. Verified in the output:

   ```
   :where(:root),…[data-theme=tabletopper]              → color-scheme:light
   @media (prefers-color-scheme:dark){:root:not([data-theme])} → color-scheme:dark
   ```

5. **Linked `app.css` first** in `base.templ`, ahead of `core.css`.

### Deviations from the sketch above

**`--sheet-muted` is not `--color-neutral-content`.** `neutral-content` is text
drawn *on* `--color-neutral` fills, not "muted body text". Pairing a light muted
brown on a dark grey fill is unreadable. `neutral` / `neutral-content` are Brixi
`grey-700` / `grey-200`; muted text is `text-base-content/60`, which is DaisyUI's
own idiom for it.

**Dark `--color-base-300` is not `--sheet-rule`.** The dark value of `--sheet-rule`
is `hsl(33.82 83.33% 74.12% / 0.3)` — a translucent gold intended for 1px
borders, unusable as a surface fill. Dark base-300 is `#3a3f47`.

**`--depth: 0` and `--noise: 0`.** These add DaisyUI's glossy bevel and grain to
buttons and cards, which fights the flat parchment panels the sheet already uses
(`--panel-shadow`, `--panel-blur`).

`--color-accent` is Brixi `primary-700` / `primary-300` — the exact colors
`link.css` uses today — so the link blue survives as a theme token once
`link.css` goes.

### Verified: rendered, not reasoned

`EditCharacter` and `Homepage` were rendered to static HTML through the real
templ components, then screenshotted in headless Chromium before and after
linking `app.css`, in both schemes.

Computed styles were diffed across 22 selectors on the character sheet — `html`,
`body`, `.sheet-section`, `.bttn`, `.input`, `.input label`, `skill-row`,
`.sheet-title`, and the rest. **The only element that changed is `html` itself:**

| Property on `html` | Before | After (light) | After (dark) |
|---|---|---|---|
| `background-color` | `rgba(0,0,0,0)` | `rgba(249,247,240,.87)` | `rgba(40,44,51,.87)` |
| `color` | `rgb(0,0,0)` | `#2f2418` | `#e9ddc9` |
| `font-family` | Times New Roman | Tailwind sans stack | " |

`color` and `font-family` on `html` are invisible — `body` overrides both. The
`background-color` is the one real effect, and it is the theme working: DaisyUI
sets `--root-bg: var(--color-base-100)` on the root. Pixel measurements:

| Page / scheme | Background before | after |
|---|---|---|
| Character sheet, light | `srgb(242,236,221)` | `srgb(242,236,220)` |
| Character sheet, dark | `srgb(41,44,51)` | `srgb(40,43,50)` |
| Homepage, dark | `srgb(45,49,55)` | `srgb(36,40,47)` |

The homepage darkens by ~9/255 in dark mode. Previously `html` was transparent
and the ~0.95-alpha body gradient composited against the browser canvas; now it
composites against the theme. Everything else is within 1/255.

### Note for Phase 4: `--font-serif` dies with brixi.css

The sketch above said "`--font-serif` (Cinzel)". **Cinzel appears nowhere in the
repo.** `--font-serif` is `Georgia, Cambria, 'Times New Roman', Times, serif`
and it is defined by **`brixi.css`** — which Phase 4 deletes. Four stylesheets
read it: `assets-page.css`, `character-cards.css`, `character-sheet.css`,
`spell-slots-table.css`. Re-declare it (in `core.css` or as a Tailwind
`@theme --font-serif`) in the same commit that deletes Brixi.

> **Phase 4 correction:** `--font-serif` was 1 of **66** variables brixi.css
> owned and this app reads. See "Correction 1" under Phase 4. It is now in
> `server/public/css/tokens.css` with the other 61.


### DECIDED: Tailwind's spacing scale

The theme **does not** override `--spacing`. Tailwind's default
(`1 unit = 0.25rem`) stands, and Brixi's values are converted ×4 in Phase 4.

Rejected alternative: remapping `--spacing` to `1rem` to preserve Brixi's
meaning. That would have made every DaisyUI example and every Tailwind snippet
pasted for the life of the project 4× off — permanent friction to avoid a
one-time scripted conversion.

Consequences:

- Neither theme block carries a `--spacing` line. (Confirmed in the shipped
  `app.css`: `--spacing:.25rem` in the `theme` layer, untouched.)
- The ×4 conversion (Appendix A) is a firm Phase 4 deliverable, not an option.
- `--radius-box` / `--radius-field` are unaffected — radius is not on the
  spacing scale, so `radius-0.5` → `rounded-lg` still holds.
- DaisyUI's own component padding now agrees with your utilities, since both
  sit on the stock scale.


## Phase 4 — Migrate Brixi classes ✅ DONE

`brixi.css` is deleted. 420KB and 5,375 `!important` declarations gone.

### What was done

1. **Deleted the dead lightswitch component** — `lightswitch.js`, `lightswitch.css`,
   and their two tags in `base.templ`. `lightswitch-component` was bound but never
   instantiated. This removed 5 of Appendix C's 8 JS-only classes before any
   migration work, exactly as that appendix predicted.

2. **Extracted `server/public/css/tokens.css` (3.5KB, 62 custom properties).**
   See the correction below — this, not the class migration, was the real work.

3. **Migrated all 74 Brixi classes** across 9 templates and 7 JS components,
   in one scripted simultaneous pass (sequential `sed` would have double-applied:
   `mb-0.25`→`mb-1` collides with `mb-1`→`mb-4`).

4. **Rewrote the 12 surviving `flex="..."` attributes** as class strings.
   `[flex]` was `display:flex`; the keywords map 1:1 onto Tailwind.

5. **Wrapped `normalize.css` in `@layer base`.** See the second correction below —
   without this the entire migration would have been silently inert.

6. Swapped the `brixi.css` link for `tokens.css` and deleted the file.

### ⚠️ Correction 1: the plan found 1 of the 66 variables brixi.css owned

The Phase 3 note above flagged `--font-serif` as "the" landmine. It was one of
**66**. `brixi.css` declared 234 custom properties on `:root`; 62 of them are
read through `var()` by 24 of this app's own stylesheets — `--grey-*`,
`--primary-*`, `--danger-*`, `--success-*`, `--warning-*`, `--white`,
`--font-sm/md/xs/medium/bold`, `--ease-in`, `--ease-in-out`, `--focus-ring`,
`--input-border`, `--button-shadow`, `--bevel`, `--shadow-black-md`,
`--font-sans-serif` (read by `core.css` for `body`), `--font-serif`.

Deleting the file without them fails **silently and totally**: every one of those
`var()` reads resolves to nothing, the declaration falls back to its initial
value, and buttons lose every colour, inputs lose their borders, `body` loses its
font. Nothing errors.

They are lifted verbatim into `tokens.css`, transitively closed (`--focus-ring`
and `--input-border` reference other tokens). All 62 were single `:root`
declarations with no dark-mode variants, so it is a flat copy. `tokens.css` is
deleted piecemeal in Phase 5 as each component that reads it is replaced.

Incidental: `--font-snug`, `--line-snug` and `--loading-bar-shadow` are read by
`toast.css`, `snackbar.css` and `soft-loading.css` and were **never defined by
anything**, brixi included. Pre-existing, unrelated to this migration.

### ⚠️ Correction 2: Tailwind was never actually working

`normalize.css` has a Tailwind-preflight-style tail appended to the stock
normalize v8.0.1. Unlayered, it beat everything in `@layer utilities`:

| normalize.css rule | killed |
|---|---|
| `*,::after,::before{position:relative}` | every `.absolute`, `.fixed` |
| `main{display:block}` | `.flex` on the app's only `<main>` |
| `h1..h6{font-size:inherit;font-weight:inherit}` | every `text-*` and `font-*` on a heading |

Measured in a headless render, before the fix:

```
main display : block   (want flex)      h1 : 16px/400  (want 32px/700)
div position : relative (want fixed)    h2 : 16px/400  (want 20px/500)
```

This was **already true in Phases 1–3** — Brixi's `!important` was doing all the
work, so the Tailwind utilities added in those phases had no effect and nobody
could tell. Deleting Brixi is what exposed it.

Fixing it is one line: a reset belongs in `@layer base`, the same layer Tailwind
puts its own preflight in. `normalize.css` is now `@layer base { … }`, and stays
after `app.css` in link order so it still wins *within* that layer, as before.

This is the Phase 3 cascade-layer finding again, in the other direction: there,
layered DaisyUI losing to unlayered app CSS was protection; here, layered
Tailwind losing to an unlayered reset was silent breakage.

### The `!important` inversion, and the one place it still bites

Every Brixi utility was `!important`, so anywhere a utility and the app's own CSS
set the same property, Brixi won. With Brixi gone the app's unlayered CSS wins.
A full computed-style sweep of every element on every renderable page — comparing
each element against a probe carrying the same classes in a neutral container —
found exactly one case not fixed by layering the reset:

- `button.css .bttn{position:relative}` beat `.absolute` on the copy button
  inside `input.js`. Fixed with Tailwind's important modifier: `absolute!`.

Deliberately **not** reverted: inside the character sheet, `character-sheet.css`
now wins where Brixi's `!important` used to override it. Table headers render at
`.71rem`/700/uppercase/`--sheet-muted` instead of `.875rem`/500/zinc-800. That is
the hand-written parchment design finally taking effect. **First thing to look at
when spot-checking.**

### Corrections to Appendix B's mapping

Verified against brixi.css's own custom properties rather than assumed:

| Brixi | value | plan said | actually |
|---|---|---|---|
| `font-lg` | 1.25rem | `text-lg` | **`text-xl`** — `text-lg` is 1.125rem |
| `font-2xl` | 2rem | `text-2xl` | **`text-[2rem]`** — no exact Tailwind step (2xl=1.5, 3xl=1.875) |
| `font-grey-*` | zinc | `text-neutral-*` | **`text-zinc-*`** — brixi's palette *is* Tailwind's: grey=zinc, primary=blue, danger=rose, success=emerald, warning=amber, exactly |

`text-[2rem]` is the one arbitrary value introduced. It is on three `<h1>`s
(tos, privacy, server-error) and can be snapped to `text-3xl` (1.875rem) in
cleanup if the 2px is not worth the escape hatch.

Also note Tailwind's `text-*` utilities set `line-height` as well as `font-size`;
Brixi's `font-*` set only `font-size`. Vertical rhythm on migrated text shifts
slightly.

### Also corrected: the ~110 attribute usages were 13, not 110

The plan listed `flex` `kind` `color` `size` `shape` `icon` `sfx` `tooltip` as
needing hand rewriting. Only `flex` (12 uses) is Brixi's. The rest are owned by
files that survive Phase 4:

| attribute | owner | action |
|---|---|---|
| `kind` `color` `size` `shape` `icon` | `button.css` (41KB) | none — retired in Phase 5 |
| `sfx` | `soundscape.js` | none — a JS hook, not CSS |
| `tooltip` | `tooltipper.js` | none — read in JS, styles `<tool-tip>` |
| `position` | nothing | deleted, it was already inert |
| `flex` | **brixi.css** | rewritten as classes |

Two `flex=` values were doing nothing before the migration and were preserved
as-is rather than "fixed":

- `assets.templ` `flex="row nowrap items-center space-between"` — `space-between`
  is not a Brixi keyword (it is `justify-between`). Renders `flex-start`.
  **Almost certainly a bug; left alone to keep this phase non-visual.**
- `assets.templ` `flex="row nowram items-center"` — typo. Harmless: `nowrap`
  is the initial value anyway.

### Also corrected: the `h-6`/`w-6` prediction

The trap section predicted the Heroicons `h-6 w-6` on SVGs in `alerts.js`,
`select.js` and `spell-slots-table.js` would start sizing to 1.5rem once Tailwind
was linked. Measured: they stay 16px. The components' own unlayered CSS sizes
those SVGs and beats the layered utility. No change.

### Verified

```
make css                      106ms, 39,949 bytes, daisyUI 5.7.28 banner
templ generate / go build / go vet / go test ./...     all clean
```

- Every class literal in every template and JS component resolves to a rule.
  The 14 that do not are pre-existing: tabler icon classes (`icon-tabler-*`),
  FontAwesome leftovers (`fa-times`, `svg-inline--fa`), and JS-only hooks
  (`js-notification-close`). None are Brixi.
- Computed-style sweep across `/`, `/tos`, `/privacy`, `/error`, `/sign-in`,
  `/sign-up`, `/characters/new`: **zero utilities overridden** after the two
  fixes above. Remaining diffs are probe artifacts (percentage heights, and
  elements inside the `x-cloak` modals, which are `display:none`).
- CSS audit: 25 files on disk, 25 linked, none orphaned, none missing.

### Left for the cleanup round

- **8 stylesheets load twice** — `button` `input` `monster-info-table`
  `saving-throws-table` `select` `skills-table` `spell-slots-table` `toast` are
  linked in `base.templ` *and* injected at runtime by `env.css([...])`. Same
  content, so it is redundant rather than harmful. All 8 belong to components
  Phase 5 deletes.
- `assets.templ`'s `space-between` bug above.
- `text-[2rem]` → `text-3xl`, if wanted.
- Not exercised against the running stack: the `hx-status:422` form path, the
  delete-confirm modal, and toasts on a redirecting response.


## Phase 5 — DEFERRED

**SSR the client-rendered components.**

Not scheduled. Recorded here because it is the reason this migration is worth doing at all, and because its scope is larger than it first appears.

### The problem

The character sheet is not a form until JavaScript runs. The server emits:

```html
<number-input-component data-name="str" data-label="Strength" data-value="10"></number-input-component>
```

— an empty element. lit-html then creates the real `<input name="str">` client-side. All **23 form fields** the server parses in `buildCharacterFormInput` exist only because JS ran. That is an architectural inversion for an SSR + HTMX app: no progressive enhancement, and HTMX's form serialization depends on JS having already rendered the inputs.

### Scope is 7 components, not 3

| Component | Instances |
|---|---|
| `number-input-component` | 28 |
| `input-component` | 14 |
| `select-component` | 4 |
| `monster-info-table` | 6 |
| `skills-table` | 2 |
| `saving-throws-table` | 2 |
| `spell-slots-table` | 2 |

All seven follow the same pattern — empty element, `data-*` attributes, lit-html renders client-side.

**Converting only the three form components drops 5 files / 15KB and lit-html stays**, because the four tables still import it. Getting off lit-html requires all seven.

### Why it is less work than it looks

- The client-side validation being deleted is **already duplicated server-side**. `buildCharacterFormInput` returns `validationErrors`, and they render through the `hx-status:422` path wired during the htmx 4 migration.
- The field contract does not change: templ emits `<input name="str">` where JS used to create it. Same 23 names, same parsing code.
- Doing this after Phase 4 means the new markup is written in Tailwind/DaisyUI once, rather than in Brixi and then migrated.

---

## Appendix A — Spacing conversion (×4, exact)

All 25 in-use spacing/sizing classes — the complete "silent failure" set from
the trap section. Every one must be rewritten. No rounding anywhere.

| Brixi | Value | Tailwind |
|---|---|---|
| `mb-0.25` | 0.25rem | `mb-1` |
| `mb-0.75` | 0.75rem | `mb-3` |
| `mb-1` | 1rem | `mb-4` |
| `mb-1.5` | 1.5rem | `mb-6` |
| `mb-3` | 3rem | `mb-12` |
| `ml-0.75` † | 0.75rem | `ml-3` |
| `mr-0.5` | 0.5rem | `mr-2` |
| `mr-0.75` | 0.75rem | `mr-3` |
| `mr-1` | 1rem | `mr-4` |
| `mr-2` | 2rem | `mr-8` |
| `mt-0.25` | 0.25rem | `mt-1` |
| `p-1` | 1rem | `p-4` |
| `p-1.5` | 1.5rem | `p-6` |
| `p-2` | 2rem | `p-8` |
| `pb-1` | 1rem | `pb-4` |
| `pl-0.125` † | 0.125rem | `pl-0.5` |
| `pl-1` | 1rem | `pl-4` |
| `pl-3` | 3rem | `pl-12` |
| `pr-4` | 4rem | `pr-16` |
| `pt-1` | 1rem | `pt-4` |
| `pt-1.75` | 1.75rem | `pt-7` |
| `pt-2` | 2rem | `pt-8` |
| `px-1` | 1rem | `px-4` |
| `px-2` | 2rem | `px-8` |
| `max-w-1024` | 1024**px** | `max-w-[1024px]` |

Rule: **Tailwind unit = Brixi rem value × 4.**

† Reachable only through JS (Appendix C).

`max-w-1024` is the one exception to the ×4 rule — it is a pixel value, not a
rem value, so it does not convert. It is listed here because it belongs to the
same silent-failure class: Tailwind reads `1024` as 1024 spacing units.

---

## Appendix B — Remaining 44 classes

The non-spacing half of the 66 template classes. For the 8 that live only in JS,
see Appendix C.

**Identical in both — no change (9):**
`block` `inline-block` `fixed` `relative` `text-center` `w-full` `h-full` `w-screen` `h-screen`

**Typography (11)** — note Brixi's `font-` prefix covers weight *and* color:

| Brixi | Tailwind |
|---|---|
| `font-xs` / `font-lg` / `font-2xl` | `text-xs` / `text-lg` / `text-2xl` |
| `font-medium` / `font-semibold` / `font-bold` | unchanged |
| `font-grey-100` / `font-grey-700` / `font-grey-900` | `text-neutral-*` (theme) |
| `font-danger-700` | `text-error` |
| `font-primary-300` | `text-primary` |

**Borders / radius (8):**

| Brixi | Tailwind |
|---|---|
| `border-1` | `border` |
| `border-t-1` | `border-t` |
| `border-solid` / `border-t-solid` | `border-solid` |
| `border-danger-400` | `border-error` |
| `border-grey-300` / `border-t-grey-200` | `border-neutral-*` |
| `radius-0.5` | `rounded-lg` (0.5rem) |

**Positioning / misc (9):**

| Brixi | Tailwind |
|---|---|
| `t-0` / `b-0` / `l-0` | `top-0` / `bottom-0` / `left-0` |
| `x-center` | `left-1/2 -translate-x-1/2` |
| `max-w-full` | `max-w-full` |
| `max-w-1024` | `max-w-[1024px]` — **see Appendix A**, silent failure |
| `bg-white` / `bg-grey-100` | `bg-base-100` / `bg-base-200` |
| `shadow-black-sm` | `shadow-sm` |

**Dark variants (7)** — `dark:` works in both; rename the color half:
`dark:bg-grey-900/87` `dark:bg-grey-950/60` `dark:border-grey-800` `dark:border-t-grey-800` `dark:font-danger-400` `dark:font-grey-100` `dark:font-grey-300`

---

## Appendix C — Brixi classes reachable only through JS

Found during Phase 1, when `public/js/**/*.js` was added as a Tailwind source.
These are emitted by lit-html components, so they never appear in a `.templ`
file and were absent from the original 66-class survey.

| Brixi | Value | Tailwind | Emitted by |
|---|---|---|---|
| `absolute` | `position:absolute` | `absolute` (no change) | `input.js` |
| `r-0` | `right:0` | `right-0` | `input.js` |
| `font-sm` | `var(--font-sm)` | `text-sm` | the 4 tables, `lightswitch.js` |
| `font-grey-800` | `var(--grey-800)` | `text-neutral-*` (theme) | the 4 tables |
| `pl-0.125` | 0.125rem | `pl-0.5` | the 4 tables |
| `line-snug` ‡ | `line-height:1.375` | `leading-snug` (1.375, exact) | `lightswitch.js` |
| `font-grey-500` ‡ | `var(--grey-500)` | `text-neutral-*` (theme) | `lightswitch.js` |
| `ml-0.75` ‡ | 0.75rem | `ml-3` | `lightswitch.js` |

All eight map exactly; `line-snug` → `leading-snug` matches Tailwind's 1.375 on
the nose.

Only `absolute` and `ml-0.75` are in the silent-failure set — `ml-0.75` renders
at ¼ size if missed, `absolute` is equivalent in both. The other six vanish
visibly.

### ‡ Three of these are already dead

`lightswitch.js` registers `lightswitch-component` via `env.bind(...)`, and **no
template instantiates that element** — the file is loaded by `base.templ` and
does nothing but inject `lightswitch.css`. Deleting the `<script>` and `<link>`
for it (Phase 4's CSS audit already flags `lightswitch.css` as the one
confidently dead stylesheet) drops this appendix from 8 classes to 5, and takes
the JS side of the Phase 4 danger grep to zero — `class="ml-0.75"` in
`lightswitch.js` is its only hit across all 27 JS files.

Worth doing first in Phase 4. It is a deletion, not a migration.

---

## Facts this plan rests on

Measured against the current tree, not assumed:

- 120 distinct classes in `server/templ/**/*.templ`: **66 Brixi**, 44 own component CSS, 10 undefined (mostly tabler icon classes).
- A further **8 Brixi classes** appear only in `server/public/js/**/*.js` (Appendix C), for **74 total**. The original survey scanned templates only.
- Of the 74, Tailwind 4.3.3 silently generates a rule for 42 and nothing for 32. Of the 42, **24 differ in value** — 23 at ¼ size, plus `max-w-1024` at 4× size.
- `brixi.css`: 420KB, **5,375 `!important`** across ~5,309 rules.
- Brixi spacing scale confirmed `1 unit = 1rem`; Tailwind's is 0.25rem.
- ~110 attribute-styling usages (`sfx` 24, `kind` 24, `color` 24, `flex` 12, `icon` 10, `tooltip` 9, `shape` 5, `size` 2).
- Dark mode is `prefers-color-scheme` only — no toggle, no `data-theme`.
- Components inject their own CSS at runtime via `env.css([...])`, which defeats static unused-CSS analysis.
- Tailwind's automatic source detection reaches `client/`: 137 selectors vs 62 for the explicit config, including false positives extracted from TypeScript type annotations. Hence `source(none)`.
