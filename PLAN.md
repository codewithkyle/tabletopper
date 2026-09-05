# Form controls → DaisyUI, lit-html → server-rendered

The last leg of the Brixi teardown. When it lands, `tokens.css` is deleted, the
`exclude:` list in `server/css/app.css` is empty, and the lit-html component
runtime is gone from `server/public/js`.

**Scope:** the seven interactive elements on the character sheet, plus `.link`.
Everything else in `server/public/css` is already on DaisyUI tokens.

| | today | after |
|---|---|---|
| stylesheet `<link>`s in `base.templ` | 15 | **7** |
| `<script>`s in `base.templ` | 13 | **6** |
| Brixi token reads | 234 | **0** |
| CSS deleted | — | **27,062 B** across 8 files |
| JS deleted | — | **48,194 B** across 15 modules |
| JS surviving | — | 9,344 B (`notif` · `alerts` · `toaster` · `env` · `confirm-modal` · `uuid`) |

---

## Why this is safe to do incrementally

**The `exclude:` list is the safety rail.** `app.css` currently carries:

```css
@plugin "./vendor/daisyui.js" {
  themes: caramellatte --default, coffee --prefersdark;
  exclude: input, select, label, link;
}
```

Those four names are excluded because this app already owns those class names —
`.input` is added at runtime by `input.js` (`this.classList.add("input")`),
`.select` by `select.css`, `.label` by the two `<span class="label">`s in the
skills and saving-throws tables, `.link` by `link.css`. While a name sits in
`exclude`, DaisyUI emits **no CSS at all** for it, so the old and new worlds
cannot collide.

Each phase below removes exactly one name from that list, in step with the
markup that owned it. Nothing needs a big-bang cutover, and every phase is
independently testable and independently revertible.

**The server needs almost no changes for Phases 1–6.** The Go controller already
parses everything with `r.PostFormValue("name")` / `r.PostForm["features-name"]`
— it has never known these were custom elements. The lit-html components render
real `<input name=…>` into light DOM, so the wire format is already plain
`application/x-www-form-urlencoded`. Replacing them with server-rendered
`<input>` changes nothing the handler sees. Only Phase 7 (spells) proposes a
controller change, and it is optional.

**Nine of the ten `data-*` options are dead.** `input.js` supports `icon`,
`instructions`, `datalist`, `readOnly`, `autofocus`, `disabled`, `maxlength`,
`minlength`, `autocomplete` and `autocapitalize`. Grepping every `<input-component>`
in `templ/`, the server app uses only:

```
input-component          data-name data-label data-required data-value data-placeholder
number-input-component   data-name data-label data-required data-value
select-component         data-name data-label data-required data-value data-options
```

So the server-rendered replacements need five attributes, not fifteen. The
copy-to-clipboard button, the icon slot, the eye toggle, the datalist and the
whole `DISABLED` branch of both stylesheets are being deleted, not ported.

**`client/` is unaffected.** The legacy SPA has its own copies under
`client/public/js/`; nothing there imports from `server/public/js/`.

---

## Cross-cutting decisions

Settle these once in Phase 1; every later phase just applies them.

### D1 — Validation: `.validator`, not a state machine

`input.js`, `number-input.js` and `select.js` each carry a hand-rolled
`IDLING → ERROR → DISABLED` state machine driving `[state=ERROR]` selectors in
CSS, ~60 lines per component. DaisyUI 5 does all of it in CSS:

```html
<input class="input validator" required minlength="2" />
<p class="validator-hint">This field is required.</p>
```

`.validator:user-invalid` flips `--input-color` to `--color-error` and reveals
the sibling `.validator-hint`. `:user-invalid` only matches **after the user has
interacted**, which is exactly the validate-on-blur behaviour the current
components implement by hand. Native `required` / `minlength` / `min` / `max`
cover every rule the JS enforces.

Net: three state machines, three sets of `[state=ERROR]` rules, and the
`text-rose-700` hardcodes all delete. Zero JS.

### D2 — Field label: `<label class="label">`, with one override

DaisyUI's documented form shape maps cleanly onto our sheet sections:

```html
<label class="label" for="name">Character Name</label>
<input id="name" name="name" class="input validator w-full" required />
<p class="validator-hint">This field is required.</p>
```

**`.label` ships at `color-mix(in oklab, currentcolor 60%, transparent)`, which
fails AA on our panels** — measured 3.26:1 caramellatte, 3.39:1 coffee. This is
the same finding as `text-base-content/60` from the card migration. Same fix,
one unlayered rule:

```css
.label { color: color-mix(in oklab, currentcolor 75%, transparent); }
```

→ 4.68:1 and 4.57:1. Matches the `/75` standard already adopted for muted body
text elsewhere.

### D3 — `.input` needs `w-full`

DaisyUI sets `width: clamp(3rem, 20rem, 100%)` on `.input`, `.select` and
`.textarea` — a **20rem cap**. In the narrow grids (abilities, 6 columns) 100%
wins and it doesn't matter. In the 2-column Proficiencies grid on a wide screen
the cell exceeds 20rem and the field would stop short. `w-full` on every field.

### D4 — Field boundary contrast: ship DaisyUI's default, decide after seeing it

DaisyUI draws the field border with
`--input-color: color-mix(in oklab, var(--color-base-content) 20%, transparent)`.
Measured against the field's own `base-100` fill:

| | caramellatte | coffee |
|---|---|---|
| DaisyUI default (base-content @20%) | 1.43 | 1.44 |
| base-content @55% | 3.00 | 2.98 |
| WCAG 1.4.11 (UI component boundary) | 3.0 | 3.0 |

Today's Brixi border is `--grey-300` on white = 1.65, so this is **not a
regression** — the boundary has always been faint. Per the standing rule, ship
DaisyUI's default and look at it. If it reads too soft on the frosted panel, the
override is three lines, and it has to dodge the validator or it will clobber
the error colour:

```css
.input:not(:user-invalid, :has(:user-invalid)),
.select:not(:user-invalid, :has(:user-invalid)),
.textarea:not(:user-invalid, :has(:user-invalid)) {
  --input-color: color-mix(in oklab, var(--color-base-content) 55%, transparent);
}
```

### D5 — `.validator-hint` fails AA under caramellatte. Open.

`--color-error` on the `bg-base-200/85` panel:

| | caramellatte | coffee |
|---|---|---|
| `--color-error` as shipped | **2.55** | 8.19 |

Caramellatte's error is `oklch(70% 0.191 22.216)` — a bright coral, and error
text is the one thing that must be readable. This is an upstream weakness in
DaisyUI's own theme, not something we introduce; today's `input.css` uses
`--danger-700` (#BE123C) for the same text and passes.

Three options, none free:

1. **Override `--color-error` in caramellatte only** → also needs
   `--color-error-content` overridden, because caramellatte's error-content is
   `oklch(39% 0.141 25.723)`, a *dark* red that only works on a bright fill.
   Two tokens, and it visibly changes the three `btn-error` buttons.
2. **`color-mix(in oklab, var(--color-error) 55%, var(--color-base-content))`
   on `.validator-hint` only** → surgical, theme-neutral, but only reaches
   4.30:1 under caramellatte. Still short, and it muddies the hue.
3. **Ship it and accept 2.55** for now.

Recommend deferring to Phase 2, when there is a real error message on screen to
judge. Flagging it here so it isn't discovered late.

### D6 — Row repeaters: htmx fragments, not Alpine

Phases 6 and 7 need "add another row". Two ways:

- **htmx** — `hx-get` a templ-rendered fragment, `hx-swap="beforeend"`. Row
  markup lives in exactly one place (a templ component), which is the whole
  point of the move to SSR. Costs a round trip per row and one new route.
- **Alpine `x-for`** — no round trip, but the row markup has to be duplicated
  into a `<template>` and kept in sync with the server-rendered version.

Recommend **htmx**, because single-source markup is the reason for this work.
Delete stays client-side and needs no server: one Alpine `@click` that removes
the closest row.

---

## Phase 1 — `link`

**Warm-up. Proves the exclude-list mechanic on one line of markup.**

- One usage in the whole server app: `server-error.templ:21`.
- `link.css` is 491 B / 9 Brixi reads of `--primary-{200..800}` and `--focus-ring`.
- DaisyUI's `.link` is just `text-decoration: underline` + a focus ring — **it
  sets no colour at all**. So `link` alone inherits `currentColor`; the old blue
  needs `link-primary`, or drop the colour and let it read as body text.

**Do:** `class="link"` → `class="link link-primary"` (or plain `link` — decide
on sight). Delete `public/css/link.css`, drop its `<link>` from `base.templ`,
remove `link` from `exclude`.

**Test:** visit `/error`, click the GitHub link, tab to it and check the focus ring.

---

## Phase 2 — text input  (`input-component` → `<input class="input">`)

The big one for establishing the pattern. 14 usages across `new-character.templ`
and `edit-character.templ`.

**New templ component**, `templ/pages/form-field.templ`:

```templ
templ textField(name, label, value, placeholder string, required bool) {
	<div>
		<label class="label mb-1" for={ name }>{ label }</label>
		<input
			id={ name }
			name={ name }
			type="text"
			class="input validator w-full"
			value={ value }
			placeholder={ placeholder }
			required?={ required }
		/>
		<p class="validator-hint">This field is required.</p>
	</div>
}
```

**Do:**
- Replace all 14 `<input-component>` with `@textField(...)`.
- Remove `input` **and `label`** from `exclude` (D2 needs `.label`).
- Delete `public/css/input.css` (6,207 B, 62 Brixi reads) and its `<link>`.
- Delete `public/js/input.js` and `public/js/input-base.js`, and its `<script>`.
- Add the `.label` 75% override from D2 to `core.css`.

**Known interim wobble:** un-excluding `label` in this phase makes DaisyUI's
`.label` apply to the `<span class="label">` in the skills and saving-throws
tables until Phases 5–6 remove them. Of the properties DaisyUI sets, `color` is
overridden by the unlayered table CSS, and `display: inline-flex` is blockified
away by the flex parent — so the only live effect is `white-space: nowrap` on
skill and save names. Worth a glance at "Animal Handling" in the 3-column grid
while testing; it is temporary either way.

**Also fixed here:** `input.js` calls `env.css(["input","button","toast"])`, and
`button.css` was deleted in the card migration — so every page with a text field
currently fires a 404 for `/css/button.css`. It resolves through the `onerror`
handler, so nothing breaks; it just dies with the component.

**Test:** create a character. Leave "Character Name" blank and submit — the
validator hint should appear and the border should go error-coloured, without
the page reloading. Confirm the saved values round-trip on the edit page.

---

## Phase 3 — number input  (`number-input-component` → `<input type="number">`)

28 usages — the six ability scores plus nine core stats, on both pages. Same
component shape as Phase 2 with `type="number"` and `min`/`max`/`step`.

The JS enforced `min: 0, max: 9999` and coerced with
`value.replace(/[^\d\.\-]/g, "")`. Native `type="number"` + `min`/`max` +
`.validator` covers all of it, and the browser's own spinners replace the
hand-built ones.

`--depth: 1` on caramellatte gives `.input` an inset shadow; DaisyUI also nudges
`::-webkit-inner-spin-button` by `-10px`. Worth eyeballing on the 6-across
ability grid where the fields are narrowest.

**Do:** add `numberField(...)`, replace all 28, delete `public/js/number-input.js`
and its `<script>`. No stylesheet to delete — it shared `input.css`, already gone.

**Test:** the abilities row at desktop, at `max-[1160px]` (3-col) and at
`max-[640px]` (2-col). Type letters into an ability score; type `-5`; submit.

---

## Phase 4 — select  (`select-component` → `<select class="select">`)

4 usages: Alignment and Size, on both pages. `data-options` is a JSON blob
parsed client-side; server-rendered it becomes a Go slice ranged over in templ.

DaisyUI's `.select` **draws its own chevron** as a `background-image` of two
linear-gradients, so the `<i class="selector">` SVG and its positioning rules
all go. `padding-inline-end: 1.75rem` reserves the space.

**Do:**
- A `selectField(name, label string, options []Option, value string, required bool)`
  component; move the 16-entry alignment list and the 6-entry size list into Go
  (`templ/pages/character-options.go`) so both pages share one source.
- Remove `select` from `exclude`.
- Delete `public/css/select.css` (6,324 B, 56 Brixi reads) and its `<link>`.
- Delete `public/js/select.js` and its `<script>`.

**Test:** both dropdowns on both pages; confirm the edit page pre-selects the
saved value (this is where a server-rendered `selected` attribute replaces the
JS `?selected=${...}` binding — the most likely place for a bug).

---

## Phase 5 — saving throws  (`saving-throws-table`)

**The first table, and the easiest thing in this document.** Six rows, fixed
list, no add, no delete, no reordering. The entire component is a `for` loop
over a constant.

Field names are `saving_throws-str` … `saving_throws-cha`; the Go side
(`marshalSavingThrowsPayload`) scans `PostForm` for the `saving_throws-` prefix,
so nothing server-side changes.

`SavingThrowsJSON string` on `EditCharacterPageData` becomes
`SavingThrows map[string]int` — the JSON blob stops being a transport to the
browser and gets unmarshalled in the controller instead.

**Do:** a `savingThrowsTable` templ component; row is
`<div class="flex items-center justify-between gap-2 rounded border-2 border-base-300 p-2">`
with the name/abbr stack and a `<input type="number" class="input input-sm w-14 text-right">`.
Delete `public/css/saving-throws-table.css` (19 Brixi reads),
`public/js/saving-throws-table.js`, and both tags from `base.templ`.

**Test:** set a few bonuses, save, reload the edit page, confirm they persist.

---

## Phase 6 — skills  (`skills-table`)

Mechanically identical to Phase 5 with 18 rows instead of 6, a 3-column grid,
and an ability abbreviation instead of a stat abbreviation. Prefix
`skills-`; `marshalSkillsPayload` unchanged.

**This phase retires the last `<span class="label">`**, which closes out the
interim `white-space: nowrap` wobble noted in Phase 2.

**Do:** `skillsTable` component, delete `public/css/skills-table.css`,
`public/js/skills-table.js`, and both tags.

Note: the `<h4>` heading in both table components is dead code on the server —
every call site passes `data-label=""`, so the falsy branch renders `null`. It
does not need porting.

**Test:** the 3-column grid at each breakpoint; confirm long names like
"Animal Handling" and "Sleight of Hand" wrap rather than overflow.

---

## Phase 7 — features / equipment / resources  (`monster-info-table`)

Three instances of one repeater: a name `<input>` + description `<textarea>` per
row, an add button, a delete button per row.

**The wire format needs no index.** Go zips `PostForm["features-name"]` against
`PostForm["features-value"]` in document order, so every row emits the *same*
two field names. That makes both operations trivial:

- **add row** — `hx-get="/characters/fragments/info-row?field=features"`,
  `hx-swap="beforeend"`, `hx-target` the rows container. One new route, one
  handler, one templ component shared with the initial server render.
- **delete row** — no server involved:
  `x-on:click="$el.closest('[data-info-row]').remove()"`. Alpine is already
  loaded for the modals.

**Do:** `infoRowsTable` + `infoRow` components; `FeaturesJSON`/`WeaponsJSON`/
`ResourcesJSON` become `[]InfoRow`. Delete `public/css/monster-info-table.css`
(25 Brixi reads), `public/js/monster-info-table.js`, and both tags. The
`btn btn-dash btn-warning` add button and the delete icon carry over as-is —
they are already DaisyUI.

The textarea gets `class="textarea w-full"`; DaisyUI's `.textarea` has the same
20rem clamp as `.input` (D3).

**Test:** add three rows, fill two, delete the middle one, save. The blank row
should be dropped by the existing `name == "" && value == ""` guard, and the two
survivors should come back in order.

---

## Phase 8 — spellcasting  (`spell-slots-table`)

**The largest component in the app** — 13,836 B of JS and 4,738 B of CSS, 38
Brixi reads. Ten level groups, each with slots/used number fields and a list of
spell cards; each card has 7 fields including a `<select>` of the 8 schools.

Field names *are* indexed here: `spells-level-3-spell-0-name`. Two ways forward:

**Option A — keep the wire format.** `marshalSpellSlotsPayload` sorts by index
and drops all-empty spells, so **indices only need to be unique and increasing;
gaps are fine.** That means deleting a row needs no renumbering, and adding one
just needs a monotonically increasing counter, which the add-row `hx-get` can
carry as a query param. No controller change.

**Option B — make spells order-based like Phase 7.** Every card in a level emits
the same field names (`spells-level-3-name`, `-components`, …) and Go zips seven
parallel slices. This deletes `spellEntryFieldPattern`, the nested
`map[int]map[int]*spellPayload`, and the `sort.Ints` — the parser gets
*simpler* — and the client becomes fully stateless. Costs a controller change
and a careful test.

**Recommend Option A for the first pass** (isolates the risk to the templates),
with Option B as a follow-up once the markup is settled.

**Do:** `spellSlotsTable` / `spellLevel` / `spellCard` components; one add-spell
route per the Phase 7 pattern; `SpellSlotsJSON` becomes `[]SpellLevel`. Delete
`public/css/spell-slots-table.css`, `public/js/spell-slots-table.js`, and both
tags. The school dropdown reuses `selectField` from Phase 4.

Also: `spell-slots-table.css` is the one table sheet with `.lvl` styling and its
own `prefers-color-scheme` block already partially unwound in the card
migration — check nothing depends on the leftovers before deleting.

**Test:** the whole spellcasting section end to end. Add spells at levels 0, 3
and 9; delete a middle one at level 3; set slots > used and used > slots (the
parser clamps `used` to `slots`); save; reload.

---

## Phase 9 — delete `tokens.css` and the lit-html runtime

Everything above leaves these last threads.

**Two Brixi reads that are not form controls** and must be fixed here or the
deletion is silently lossy:

1. **`core.css`: `body { font-family: var(--font-sans-serif) }`.** That token is
   Brixi's Rubik stack, and Rubik is loaded by the `@import` at the top of
   `core.css`. With `tokens.css` gone the declaration becomes invalid at
   computed-value time and the body **silently inherits Tailwind's default sans
   stack — losing Rubik across the whole app with no error.** Fix: declare the
   stack as `--font-sans` in the `@theme` block of `server/css/app.css` and let
   Tailwind's preflight apply it.

2. **`--ease-in` in `core.css` (`.htmx-indicator`) and `bootstrap.css`
   (`#wood-img`).** Brixi's `--ease-in` is `cubic-bezier(0, 0, 0.2, 1)`, which
   is byte-identical to Tailwind's `--ease-out`. Swap for the `ease-out`
   utility/token. (`#wood-img` has no references anywhere in `templ/` or
   `public/js/` — it can just be deleted.)

**One collision worth knowing about:** `tokens.css` also declares `--font-serif`,
and so does Tailwind. Both land on `:root`, but `tokens.css` is unlayered and
Tailwind's `@theme` is in `@layer theme` — so **Brixi's value is what
`font-serif` resolves to today**, everywhere it is used (sheet headings, card
titles, the asset-manager rename field). Deleting `tokens.css` hands the token
back to Tailwind, whose stack is the same list with `ui-serif` prepended.
Effectively invisible, but it is a real change and this is where it happens.

**Then delete:**

- `public/css/tokens.css` and its `<link>` — Brixi is gone.
- `public/js/`: `supercomponent.js`, `component.js`, `lit-html.js`,
  `unsafe-html.js`, `directive-d639fc45.js`, `general.js`, `cache.js` — the
  entire component runtime, now unreferenced.
- `env.js` survives (`notif.js` needs it) but its `css()` lazy-loader becomes
  dead code; strip it.
- The `exclude:` list in `app.css` is now empty — delete the line and the
  comment block above it that explains why it existed.

**Test:** every page. Specifically confirm body text is still Rubik and headings
are still the serif — those are the two things that fail silently.

---

## Measurements

All taken from the built `server/public/css/app.css`, converting oklch → sRGB and
computing WCAG contrast. Panel = `bg-base-200/85` composited over the base-100
desk (`#FEEDD7` caramellatte, `#1F161E` coffee).

| | caramellatte | coffee | needs |
|---|---|---|---|
| field text — base-content on base-100 | 9.18 | 6.71 | 4.5 |
| `.label` @60% (DaisyUI default) | **3.26** | **3.39** | 4.5 |
| `.label` @75% (D2 fix) | 4.68 | 4.57 | 4.5 |
| `fieldset-legend` — base-content on panel | 8.51 | 7.14 | 4.5 |
| placeholder — base-content @50% | 2.67 | 2.69 | 4.5 † |
| field border @20% (DaisyUI default) | **1.43** | **1.44** | 3.0 |
| field border @55% (D4 option) | 3.00 | 2.98 | 3.0 |
| `.validator-hint` — `--color-error` on panel | **2.55** | 8.19 | 4.5 |
| field fill (base-100) vs panel | 1.08 | 1.06 | — |

† Placeholders are exempt from AA as non-essential text, and DaisyUI's 50% is
its documented default. Noted for completeness, not flagged as a defect.

## Open decisions

- **D4** — accept DaisyUI's faint field border, or override to 55%? Decide by
  looking at Phase 2.
- **D5** — how to fix `.validator-hint` under caramellatte (2.55:1). Three
  options above; recommend deciding in Phase 2 with a real error on screen.
- **D6** — htmx fragments vs Alpine `x-for` for the two repeaters. Recommend htmx.
- **Phase 8** — keep the indexed spell wire format (A) or move to order-based (B).
  Recommend A first, B as a follow-up.
