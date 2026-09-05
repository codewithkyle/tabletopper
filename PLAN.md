# Form controls → DaisyUI, lit-html → server-rendered

The last leg of the Brixi teardown. When it lands, `tokens.css` is deleted, the
`exclude:` list in `server/css/app.css` is empty, and the lit-html component
runtime is gone from `server/public/js`. The middle of those three is done: the
list emptied in Phase 6.

**All four components are now server-rendered.** Only the Phase 9 cleanup is
left: `tokens.css`, the lit-html runtime, and the three Brixi reads that survive
in `core.css` and `bootstrap.css`. The `<script>` count is already at its target
— everything still in `base.templ` stays.

**Scope:** the seven interactive elements on the character sheet, plus `.link`.
Everything else in `server/public/css` is already on DaisyUI tokens.

| | at plan time | now (after Phase 8) | after |
|---|---|---|---|
| stylesheet `<link>`s in `base.templ` | 15 | **8** | **7** |
| `<script>`s in `base.templ` | 12 † | **5** | **5** † |
| Brixi token reads | 234 | **3** | **0** |
| CSS deleted | — | 23,558 B across 7 files | **27,062 B** across 8 files |
| JS deleted | — | 35,931 B across 8 modules | **48,194 B** across 15 modules |
| JS surviving | — | — | 9,344 B (`notif` · `alerts` · `toaster` · `env` · `confirm-modal` · `uuid`) |

† Counted from `base.templ`; the original figures of 13 and 6 were off by one
each. `alerts`, `toaster`, `env` and `uuid` survive as ES module imports of
`notif.js`, not as their own `<script>` tags.

---

## Why this is safe to do incrementally

**The `exclude:` list is the safety rail.** `app.css` currently carries:

```css
@plugin "./vendor/daisyui.js" {
  themes: caramellatte --default, coffee --prefersdark;
  exclude: input, select, label, link;   /* now just: label */
}
```

Those four names were excluded because this app already owns those class names —
`.input` was added at runtime by `input.js` (`this.classList.add("input")`),
`.select` by `select.css`, `.label` by the two `<span class="label">`s in the
skills and saving-throws tables, `.link` by `link.css`. While a name sits in
`exclude`, DaisyUI emits **no CSS at all** for it, so the old and new worlds
cannot collide.

`link` left in Phase 1, `input` in Phase 2, `select` in Phase 4 and `label` in
Phase 6. **The list is now empty and the safety rail is spent** — Phases 7 and 8
have no name left to hide behind, because the components they retire never owned
a DaisyUI class name in the first place.

Most phases removed exactly one name, in step with the markup that owned it.
Phase 5 was the exception and removed none: the `<span class="label">` it
retired was one of the *two* owners of `.label`, so `skills-table` held the name
alone until Phase 6 took it. Nothing needs a big-bang cutover, and every phase is
independently testable and independently revertible.

**The server needs almost no changes for Phases 1–6.** The Go controller already
parses everything with `r.PostFormValue("name")` / `r.PostForm["features-name"]`
— it has never known these were custom elements. The lit-html components render
real `<input name=…>` into light DOM, so the wire format is already plain
`application/x-www-form-urlencoded`. Replacing them with server-rendered
`<input>` changes nothing the handler sees. That held through Phase 6. Phases 7
and 8 are the exception: not because the wire format moved — Phase 7's did not —
but because deleting the client components meant deleting the JSON round trip
that fed them, and because the add-row fragment needs a route. Neither touches
`marshalInfoRowsPayload`.

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

## Phase 2 — text and number input  ✅ done (`b29a939`)

**Phase 3 was folded into this one.** The plan had text fields land here and the
28 number fields land in Phase 3, but that split is not possible: `number-input.js`
also ran `classList.add("input")` on its wrapper. Un-excluding `input` while any
lit-html number field was still on the page would have put DaisyUI's
`height: 2.5rem`, border, `padding-inline` and background on an element holding a
label *plus* a 36px input. `.input` is one class name shared by two components, so
they had to migrate together: **42 fields, not 14.**

**Landed:** `templ/pages/form-field.templ` with `textField` and `numberField`.
`input` left `exclude`. Deleted `public/css/input.css` (6,207 B, 62 Brixi reads),
`public/js/input.js`, `input-base.js`, `number-input.js`, and their tags.

**Departure from D2 — the field shell is `.fieldset` / `.fieldset-legend`, not
`<label class="label">`.** Better on three counts:

- **`label` stayed in `exclude`**, so the interim `white-space: nowrap` wobble
  this section used to warn about never happened. It now belongs to Phase 6,
  which is where `label` actually leaves the list.
- **No contrast override was needed.** `.fieldset-legend` is full `base-content`
  (8.51 / 7.14). D2's `.label` at 75% measures 4.68 / **4.36** — the coffee
  figure was originally recorded as 4.57, and on re-measurement it is *below*
  4.5. That override would have shipped a fail.
- The accessible name still comes from a real `<label for>`. DaisyUI's own docs
  put a `<legend>` there, which would have left the input unnamed.

The wrapper is `<div class="fieldset">`, not a real `<fieldset>`: normalize gives
fieldset `padding: .35em .75em .625em` and DaisyUI's `.fieldset` only overrides
the block axis, so a real one would inset every field from its grid cell.

**Validation is zero JavaScript.** `.validator` + `:user-invalid` replaces three
`IDLING → ERROR → DISABLED` state machines with the same timing. `.validator-hint`
is turned off with `visibility`, not `display`, so it reserves its line and
**nothing jumps** when an error appears — strictly better than the old behaviour,
which rendered the `<p>` only in the ERROR state and so did shift.

**Left as-is, on purpose:** every number field carries `max="9999"`, inherited
unchanged from `number-input.js`'s defaults — **including XP**, where a level-8
character needs 34,000. That ceiling predates this migration; the only thing that
changed is that the hint now says so out loud. Worth its own fix, not a silent
widening of scope inside a markup migration.

Also cleared: `input.js` called `env.css(["input","button","toast"])` and
`button.css` was deleted in the card migration, so every page with a text field
was firing a 404 for `/css/button.css`. It died with the component.

---

## Phase 4 — select  ✅ done

4 usages: Alignment and Size, on both pages. The `data-options` JSON blob —
duplicated verbatim in all four tags — is now two Go slices in
`templ/pages/character-options.go`, ranged over at render time.

**Landed:** `selectField(name, label string, options []Option, value string,
required bool)` in `form-field.templ`, same `.fieldset` / `.fieldset-legend`
shell as the text and number fields with `.select` in place of `.input`. Both
are 2.5rem tall and read the same `--input-color`, so a picker sits on the same
baseline as the text field beside it. `select` left `exclude`. Deleted
`public/css/select.css` (6,324 B, 56 Brixi reads), `public/js/select.js`, and
their tags.

**The chevron is DaisyUI's**, two linear-gradients in `background-image` pinned
to the trailing edge with 1.75rem of end padding reserved for them. So the
`<i class="selector">` SVG and the absolute positioning that placed it are gone.
It draws in `currentColor` — full `base-content` — where Brixi's was
`--grey-400` on white: **2.56 → 9.14 caramellatte / 6.72 coffee**, from well
under the 3.0 non-text threshold to comfortably over it.

**No `.validator` on these two.** `required` on a `<select>` is only violated
when the selected option's value is empty, and neither list has such an entry —
the browser lands on the first one, so these can never be `:user-invalid`.
`select.js`'s `validate()` had the same blind spot (it tested `value === ""`
against a value that was never empty). A `.validator-hint` here would be markup
that can never reveal itself. `required` stays on the element as the honest
declaration it is.

**Nothing else had to move with `select`.** The only other `<select>` in the app
is the spell-school picker inside `spell-slots-table`, which is a bare element
styled by `spell-slots-table select` and never carried the class; DaisyUI emits
no bare `select` selector of its own. Verified by selector diff.

**Note — the alignment typo is the wire format.** "Unaligned" has value
`unaliged`. That string is in the alignment column of every character row and
`characterToEditPageData` falls back to it, so it was kept. Correcting it is a
data migration, not a template change. Two related pre-existing behaviours,
preserved for parity: the new-character form used to pass `data-value="Unaligned"`,
which matched no option and fell through to the first entry (the same one), and a
stored alignment outside the 16-entry list selects nothing and silently saves as
"Unaligned" on the next submit.

**Test:** both dropdowns on both pages; confirm the edit page pre-selects the
saved value — this is where a server-rendered `selected` attribute replaces the
JS `?selected=${...}` binding, the most likely place for a bug.

---

## Phase 5 — saving throws  (`saving-throws-table`)  ✅ done

**The first table, and the easiest thing in this document.** Six rows, fixed
list, no add, no delete, no reordering — the whole component is a `for` over a
constant.

**Landed:** `templ/pages/saving-throws-table.templ`, holding the six-entry
`savingThrows` slice (the const array from the top of the JS) and the
`savingThrowsTable(bonuses map[string]int)` component. `SavingThrowsJSON string`
on `EditCharacterPageData` became `SavingThrows map[string]int`; the blob is
unmarshalled by `parseStatBonuses` in the controller instead of being handed to
the browser. Deleted `public/css/saving-throws-table.css` (1,729 B, 19 Brixi
reads), `public/js/saving-throws-table.js` (2,402 B), and both tags.

Field names are unchanged (`saving_throws-str` … `-cha`), so
`marshalSavingThrowsPayload` and the rest of the server were untouched — as
predicted. The `<h4>` was dead code on the server and was not ported.

**No name left `exclude`.** The stat name was the sheet's other
`<span class="label">`; it is now a plain `<label>` carrying utilities, so
`skills-table` is the sole owner of `.label` until Phase 6.

**Departure — the grid is a container query, not viewport breakpoints, and this
is the one real finding of the phase.** The panel is a cell of the sheet's
two-column layout, which collapses at 900px, so the panel is *wider* at 899px
than at 901px. No viewport breakpoint can express that. The old sheet declared
three columns unconditionally and clipped stat names in every narrow case;
measured against the real markup at three widths:

| sheet width | panel | old rule (`grid-cols-3`) | now |
|---|---|---|---|
| 1440 | 660px | 3 cols, 0 names clipped | 3 cols, 0 clipped |
| 1000 | 440px | 3 cols, **5 of 6 clipped** | 2 cols, 0 clipped |
| 900 | 390px | 3 cols, **6 of 6 clipped** | 2 cols, 0 clipped |
| 375 | 307px | 3 cols, **6 of 6 clipped** | 1 col, 0 clipped |

`@container` with `@[22rem]` / `@[33rem]` thresholds — the widths at which a cell
still holds "Constitution" at `text-sm` beside the 3.5rem field — is one
monotonic rule with no band to get wrong. Verified by headless render at 320,
375, 414, 480, 640, 900, 1000, 1160 and 1440: three columns wide, two in the
middle, one on a phone, nothing clipped and nothing overflowing anywhere. Worth
knowing for Phase 6, which has the same panel and eighteen longer names.

**Two smaller departures from the sketch above:**

- `rounded-field`, not `rounded`. It is the theme's field radius (0.5rem in both
  themes) rather than a hardcoded 0.25rem, and it matches the field sitting
  inside the row.
- **No `.validator`, and no `min`/`max`** — both carried over from the component
  this replaces. A saving throw bonus is signed, so there is no floor to declare;
  with no constraint to violate the only rule left is `step="1"`, whose one
  failure mode is a typed decimal. Wiring `.validator` for that buys a red border
  with nowhere to put a hint, and paints six `:user-valid` green borders across a
  tight grid for the normal case. The text and number fields carry it because
  they have `required`/`min`/`max` and room for a line of text.

**`parseStatBonuses` decodes `float64`, not `int`.** `type=number step="1"`
rejects a decimal on *validity* but still reports it as the value, so a
fractional bonus can reach the column — and it would fail a `map[string]int`
unmarshal outright, defaulting all six to 0. Truncating the one bad entry is the
smaller loss, and it is what the form already does with it on the next save
(`Atoi` fails on `"2.5"` and `marshalSavingThrowsPayload` falls back to 0).
`normalizeJSONObjectJSON` stays, still the path for skills until Phase 6.

**Selector diff: +740 B, nothing removed.** Everything added is a class the
markup uses — `input-sm`, `rounded-field`, `text-right`, `gap-0.5`,
`grid-cols-1`, `@container` and the two `@[…]:grid-cols-*` variants — plus one
byproduct, `.floating-label:has(.input-sm)`, which ships with `.input-sm`.

Nothing was *removed* by deleting the JS, which is worth flagging for Phase 6:
`skills-table.js` still carries the identical `text-base-content/75`,
`text-[0.71rem]` and `tracking-[0.08em]` from its own dead `<h4>`, so those
classes only actually leave the build when that file does.

**Test:** the layout is verified above at nine widths in both themes. The
save-and-reload round trip still wants a run against a real database — set a few
bonuses (including a negative one), save, reload the edit page. The wire format
is byte-identical to what the lit-html component posted, so this is a regression
check rather than a new risk.

---

## Phase 6 — skills  (`skills-table`)  ✅ done

Mechanically identical to Phase 5 with 18 rows instead of 6 and the keying
ability where the saving throws print their own abbreviation. Prefix `skills-`;
`marshalSkillsPayload` unchanged.

**Landed:** `templ/pages/skills-table.templ` (the 18-entry slice and
`skillsTable`), and — because Phase 5's row markup would otherwise have been
copied verbatim — `templ/pages/bonus-row.templ`, which now holds the row for
both grids. `SkillsJSON string` became `Skills map[string]int`, fed by the
`parseStatBonuses` written in Phase 5. Deleted `public/css/skills-table.css`
(1,645 B, 19 Brixi reads), `public/js/skills-table.js` (3,149 B), and both tags.

**Extracting the row was the right call and is worth carrying into Phase 7.**
Two tables, one row definition, two stylesheets that were byte-identical apart
from the element names — that duplication is precisely what server-rendered
markup is supposed to end. Each table now supplies only its entries and its own
grid; everything else, including the reasoning about `.validator` and the
contrast of the abbreviation, lives in one comment instead of two.

**`normalizeJSONObjectJSON` is deleted.** Skills was its last caller.

**`label` left `exclude`, and the list is now empty.** This is the one name of
the four that bought no collision fix on the way out: nothing carries
`class="label"` any more, so the 3,233 B DaisyUI emits for the group is inert.
It is emitted at all only because "label" is a word this codebase says
constantly — `data-label`, `label string`, `save.Label` — and the scanner reads
words, not class attributes. Kept anyway: `exclude` is a collision rail, not a
tree-shaker, and a `<span class="label">` written later against DaisyUI's own
documented shape should work rather than silently do nothing. Flagging it
because it is the whole cost of the phase and it is reversible in one line.

Incidentally this confirms **D2's measurement**: `.label` resolves to
`color-mix(in oklab, currentcolor 60%, transparent)` under `@supports`, exactly
the 3.26 / 3.39 the table below records. Phase 2 was right not to adopt it.

**Thresholds are wider than Phase 5's, and measured.** `@[23rem]` / `@[35rem]`
against saving throws' `@[22rem]` / `@[33rem]`, because "Investigation" is 13
characters with nowhere to break against "Constitution" at 12 — 89.0px at
`text-sm`, in Rubik, confirmed loaded in the harness rather than a fallback.
Rerunning skills at the *saving-throws* thresholds clips "Investigation" and
"Performance" at a 528px panel, which is what the extra rem buys. The 3-column
threshold sits 18px above the widest measured failure.

**The two-word names wrap, as the test above asked.** At a 572px panel (3
columns) "Animal Handling" and "Sleight of Hand" take a second line and their
grid row grows to match; nothing clips and nothing overflows. Wrapping is also
the *compact* outcome — dropping to 2 columns to avoid it would trade two taller
rows for three extra ones.

Verified across 320 / 375 / 414 / 480 / 560 / 596 / 610 / 624 / 640 / 900 /
1000 / 1160 / 1280 / 1440 in both themes: **zero clipped labels and zero
overflowing rows at every width, in both grids.**

**Selector diff: +3,493 B, nothing removed.** The two new `@[…]` thresholds and
the five `.label` rules; 3,233 B of that is `.label`. Three prose leaks were
caught and reworded away first — see the hazard note below.

Note: the `<h4>` heading was dead code on the server (`data-label=""` at every
call site) and was not ported. Phase 5 guessed its classes would leave the build
with `skills-table.js`; they do not — `monster-info-table.js` and
`spell-slots-table.js` carry the same dead `<h4>`, so `text-[0.71rem]` and
`tracking-[0.08em]` survive until **Phase 8**.

**Test:** the grid is verified above. The save-and-reload round trip still wants
a run against a real database, same as Phase 5 — the wire format is unchanged,
so it is a regression check rather than a new risk.

---

## Phase 7 — features / equipment / resources  (`monster-info-table`)  ✅ done

Three instances of one repeater — a name `<input>` plus a description
`<textarea>` per row, an add button, a delete button per row.

**Landed:** `templ/pages/info-rows-table.templ` (`InfoRow`, `infoRowsTable`,
`infoRow`, and the exported `InfoRowFragment`), `controllers.InfoRowFragment`
and the route it answers on. `FeaturesJSON` / `WeaponsJSON` / `ResourcesJSON`
became `Features` / `Weapons` / `Resources` `[]InfoRow`. Deleted
`public/css/monster-info-table.css` (2,424 B, 25 Brixi reads),
`public/js/monster-info-table.js` (3,177 B), and both tags.

**The double parse is gone.** `normalizeInfoRowsJSON` unmarshalled the stored
column, re-marshalled it into a string, and handed it to the browser in a
`data-rows` attribute so `monster-info-table.js` could parse it a second time.
`parseInfoRows` replaces the whole loop: one unmarshal, straight into the slice
the template ranges over.

### The add-row mechanic (D6, resolved: htmx)

The only genuinely dynamic operation is *add*. Delete is `.remove()`, edit is a
native field, and the initial render is the server's. So the question was only
ever where a new row's markup comes from, and there were two real answers: a
fragment route, or a `<template>` cloned client-side.

**htmx won on Phase 8, not on Phase 7.** For info rows a clone is exactly
correct with no fixup, because the field names carry no index. The spell card's
names embed the level, so a clone needs either ten copies of a seven-field card
in the page or a `__LEVEL__` placeholder rewritten in JS — client-side name
mangling, un-greppable, and the same class of logic being deleted. A route takes
`?level=N` instead. One mechanism across both phases beat the locally best one
in each.

```
GET /characters/fragments/info-row?field=features   → @infoRow("features", InfoRow{})
```

behind `RequireSession`, returning bare markup with no layout. The same `infoRow`
component serves the initial render, so **a row is defined in exactly one place**
— which is the entire reason to pay a round trip for this.

`field` is checked against the three known repeaters before anything renders. It
decides the `name` attributes the row emits, so an unvalidated prefix would be a
reflection into the wire format; an unknown one is a 404.

**Two htmx 4 behaviours make the add button safe inside the character form, and
both were read out of `public/static/htmx.min.js` rather than assumed:**

- `config.implicitInheritance` is `false`. The form's own `hx-swap="outerHTML"`
  does not reach a nested button. Under htmx 1/2 inheritance rules it would
  have, and the first added row would have replaced the entire form.
- For GET and DELETE, htmx serialises a form only when the trigger **is** the
  form: `i ? (t.matches("form") ? t : null) : (t.form || t.closest("form"))`. A
  button inside one contributes nothing, so the add request is a bare URL and
  not the whole character sheet as a query string.

Delete needs no route: `x-on:click="$el.closest('[data-info-row]').remove()"`,
with an empty `x-data` on the wrapper to open a scope. Alpine's MutationObserver
initialises rows htmx appends later, so nothing re-binds after an add.

### `required` was dropped, and it fixed a real dead end

`marshalInfoRowsPayload` drops a row whose name and description are both empty —
but `monster-info-table.js` put `required` on both fields, and htmx 4 validates
before posting. **That guard was unreachable.** Adding a row and leaving it blank
did not get it ignored; it blocked the save until you deleted the row. The test
written for this phase before it was built would have failed against the old
component for that reason.

An optional repeater whose rows are individually mandatory is not a coherent
contract, so the attribute is gone and the guard that already existed does the
job. Verified end to end: with a blank row present the form reports
`checkValidity() === true`, the blank pair is submitted, and
`marshalInfoRowsPayload` drops it. A row blank on *one* side still survives — a
name with no description is kept, which is the pre-existing behaviour.

### Markup

The shell is the fused card `monster-info-table.css` drew by hand: one bordered
box, the name and its delete button sharing a top row over a full-width
description. **DaisyUI's ghost variants are what make that a composition rather
than an override** — `.input-ghost` and `.textarea-ghost` set `border-color` to
transparent and `box-shadow` to none, so the fields sit inside the wrapper's
border instead of drawing a second one. `rounded-none` stops their own radius
from cutting the wrapper's corners; `w-full` for the 20rem clamp (D3).

Nothing had to be forced. `resize: vertical` on the textarea, which the old
stylesheet set explicitly, already computes that way. Focus is a 2px outline
*plus* a `base-100` fill in both themes — measured, and visible in both, which
was the one thing worth checking about a borderless field.

**Selector diff: +5,971 B, nothing removed.** `.textarea` and its states
(3,630 B) are a component group this app had never used; `.input-ghost` /
`.textarea-ghost` are 458 B; the rest is six utilities the markup uses
(`border-s-2`, `rounded-none`, `size-3.5`, `items-stretch`, `text-base-content/70`,
two `hover:` pairs). Deleting `monster-info-table.js` removed no selectors —
`spell-slots-table.js` still supplies every word it did, including the dead
`<h4>`'s `text-[0.71rem]` and `tracking-[0.08em]`, exactly as Phase 6 predicted.

**First phase since Phase 4 with no prose leak.** Not luck — the comments were
written against the known-dangerous vocabulary and diffed after.

**Test — run, and it passes.** Against the real templ components, the real
handler and the real htmx/Alpine bundles: start with 2 rows, add 3, fill two of
them, delete the middle added row and one original, and read the form back with
`FormData`. Result at every one of 320 / 375 / 480 / 560 / 760 / 900 / 1000 /
1280 in both themes: 5 rows after the adds, 3 after the deletes, `features-name`
and `features-value` zipping to the correct three pairs **in document order**,
zero `required` attributes, zero row overflow, no page overflow, and the name
field and delete button both 40px. The save-and-reload round trip against a real
database is still the user's, same as Phases 5 and 6.

---

## Phase 8 — spellcasting  (`spell-slots-table`)  ✅ done

**The largest component in the app** — 13,836 B of JS and 4,738 B of CSS, 38
Brixi reads. Ten level blocks, each with slots/used counters and any number of
spell cards; each card has seven fields including a picker of the eight schools.

**Landed:** `templ/pages/spell-slots-table.templ` (`Spell`, `SpellLevel`, the
schools, `spellSlotsTable` / `spellLevelBlock` / `spellCard` and the exported
`SpellCardFragment`), `controllers.SpellCardFragment` and its route, and
`templ/pages/icons.templ` — both repeaters put the identical 1.2 KB trash path in
their delete buttons, so it moved out for the same reason `bonusRow` did in Phase
6. `SpellSlotsJSON string` became `SpellLevels []SpellLevel`. Deleted
`public/css/spell-slots-table.css`, `public/js/spell-slots-table.js`, and both
tags.

### The wire format moved, and the parser got smaller

Order-based as decided: `spells-level-3-spell-0-name` became
`spells-level-3-name`, every card in a level posting the same seven keys.
`marshalSpellSlotsPayload` now reads seven parallel slices per level and zips
them; `zipSpellPayloads` stops at the shortest so a truncated post cannot index
out of range.

Gone with it: **both regexes, the `map[int]map[int]*spellPayload`, and the
`sort.Ints`.** Those existed only because the indexed format allowed gaps —
spells had to be collected into a sparse map and the keys sorted to recover
document order. Reading the slices gives that order directly. `regexp` and `sort`
are no longer imported by the controller at all.

The `-slots` / `-used` keys are per-level singletons and did not change. They
cannot collide with the seven card keys, since none of those is named `slots` or
`used`, which is what makes the level prefix safe to share.

**The stored column did not change.** Only the form field names did. Verified
against a hand-written pre-Phase-8 blob: it reads back with every spell intact,
missing levels filled in, and an over-range `slots: 120` clamped — so existing
characters need no migration. A full render → post → parse round trip is stable
across all ten levels.

### Two guards that could never fire

`normalizeSpellSlotsJSON` did what `normalizeInfoRowsJSON` did — unmarshal,
re-marshal to a string, hand it to the browser to parse again — and
`parseSpellLevels` replaces it with one unmarshal plus the level filling-in the
component used to do on load.

More interesting is the **drop-the-blank-card guard, which was unreachable for
two independent reasons.** It tested `spell.School == ""`, but the picker has no
empty option, so every card has always posted a school; and the name carried
`required`, so a blank card blocked the save outright rather than being dropped.
Exactly the Phase 7 finding, twice over. `required` is gone, and the guard now
ignores school — a card is empty when everything the user could actually have
*typed* is empty. The school is a default, not an answer.

### Why these fields do not reuse `form-field.templ`

A consequence of order-based names that lands in the templates rather than the
controller: **the field name is no longer unique, so it cannot be the id.**
`textField` and `selectField` are built on `for`/`id`, and three cards at level 3
would give three elements `id="spells-level-3-components"`. The card fields
associate their text by wrapping the control in a `<label>` instead — which is
what the lit-html component did, and is the honest cost of the wire format
decision.

They also carry no `.validator`, for the reason that kept it off the bonus rows:
these fields have no constraint, so the only state it could ever paint is
`:user-valid`, which sets `--input-color` to success. Five green borders per card
is not a validation affordance.

Related, and left alone deliberately: `textField` puts `.validator` on
non-required fields too, where it can likewise only ever paint green — Background,
Speed and Languages on the Identity panel. Making the class conditional on
`required` is a one-line change and would make `.validator` and `.validator-hint`
appear together or not at all, but it is a visual change to shipped Phase 4 work
and not Phase 8's to make.

### Markup

The card keeps the shape the stylesheet drew: name and delete button on a top
row, then a two-column grid with Components and Spell Text spanning both.
`col-span-full` rather than a prefixed span, since it is correct at one column
and at two. The grid switches on the card's own width (`@[32rem]`) rather than
the viewport's `max-width: 640px` — the panel is full width here so the viewport
rule would pass its tests, but Phases 5 and 6 established that panel width is not
a monotonic function of viewport width on this sheet, and the element query is
right for the same reason.

`min="0" max="99"` on the counters reproduces the clamp the component applied as
you typed; the browser now reports an out-of-range value instead of silently
rewriting the field, and the controller clamps again on save regardless.

The custom chevron `<i class="selector">` and its absolute positioning are gone —
DaisyUI's `.select` draws its own, as Phase 4 noted it would.

One fix on inspection: the level heading was bottom-aligned against the taller
counter group, which left a gap above it. The original centred it, and so does
this.

**Selector diff: +31 B net, and three selectors removed.** Added
`.@[32rem]:grid-cols-2`, `.col-span-full` and `.w-16`, all used by the markup.
Removed `.pl-0.5`, `.text-[0.71rem]` and `.tracking-[0.08em]` — the dead `<h4>`
that every one of these components carried and none of them rendered, which Phase
6 predicted would survive exactly until here. **The largest component in the app
cost 31 bytes of CSS**, because Phases 2 through 7 had already paid for every
class name its replacement needed.

**Test — run, and it passes.** Against the real components, the real handler and
the real htmx/Alpine bundles: add spells at levels 0, 3 and 9, fill the new
level-3 cards, delete a *middle* card at level 3, and read the form back. The
three survivors zip in document order with school and range still attached to the
right card — which is the specific thing order-based names could get wrong.
At 320 / 375 / 480 / 560 / 640 / 760 in both themes and, with real window sizes
rather than a clamp, at 900 / 1100 / 1280 / 1440 / 1920: ten level blocks, no
card overflow, no page overflow, zero `required`, the column switch landing
between a 500px and a 620px card. A blank card does not block the save; `used`
of 150 does, via `max`.

Server side, verified directly: the zip preserves order through a middle delete,
`used` clamps to `slots`, counters clamp to 0–99, the blank card is dropped, an
unknown school falls back to Evocation, and a truncated post stops at the
shortest slice instead of panicking.

The save-and-reload round trip against a real database is still the user's, but
it is now the *only* unverified step — the parse, the post and the re-parse are
covered above.

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
- `@source "../public/js/**/*.js"` in `server/css/app.css`. It was there for the
  lit-html components, which emitted class strings the templates never contained;
  all four are server-rendered as of Phase 8, so the line goes with the runtime
  below. Expect the selector diff to *shrink* when it does — that is the hazard
  note's third bullet, at scale.
- `public/js/`: `supercomponent.js`, `component.js`, `lit-html.js`,
  `unsafe-html.js`, `directive-d639fc45.js`, `general.js`, `cache.js` — the
  entire component runtime, now unreferenced.
- `env.js` survives (`notif.js` needs it) but its `css()` lazy-loader becomes
  dead code; strip it.
- ~~The `exclude:` list in `app.css`~~ — **done in Phase 6.** The line is gone;
  the comment block above it was kept and rewritten as the record of why each of
  the four names was ever on the list, since the reasons are the argument for the
  mechanic rather than for the names.

**One more silent read, found in Phase 8 while counting what was left:**
`soft-loading.css` reads `var(--loading-bar-shadow)` twice and nothing anywhere
defines it, so both `box-shadow` declarations have always been invalid at
computed-value time and the bars have never had a shadow. Not a Brixi token and
not caused by any of this — but it is the same failure mode as the
`--font-sans-serif` item above, and this is the phase that is looking at these
files. Either define it or drop the declarations.

(`toast.css` has a comment about `var(--font-snug)`, a token that never existed
either. That one was already found and removed; the note is the record of it.)

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
| `.label` @75% (D2, **not used** — see Phase 2) | 4.68 | **4.36** | 4.5 |
| `fieldset-legend` — base-content on panel | 8.51 | 7.14 | 4.5 |
| placeholder — base-content @50% | 2.67 | 2.69 | 4.5 † |
| field border @20% (DaisyUI default) | **1.43** | **1.44** | 3.0 |
| field border @55% (D4 option) | 3.00 | 2.98 | 3.0 |
| `.validator-hint` — `--color-error` on panel | **2.55** | 8.19 | 4.5 |
| field fill (base-100) vs panel | 1.08 | 1.06 | — |

† Placeholders are exempt from AA as non-essential text, and DaisyUI's 50% is
its documented default. Noted for completeness, not flagged as a defect.

## A hazard found in Phase 4 — Tailwind scans words, not class attributes

`@source "../templ/**/*.templ"` means Tailwind extracts a class candidate from
**every word in the file**. Go keywords and English prose in comments count.

- `for _, o := range options` is why **`.range` is in this build** — 9 rules for
  a slider the app does not have. Unavoidable while `range` is a Go keyword.
- A comment in `form-field.templ` that used the words "dropdown" and "list" was
  worth **6,166 bytes** (30 rules of `.dropdown`, `.list`, `.menu`, `.inline`)
  until it was reworded. That is 5.6% of the file, for two words of prose.
- It runs the other way too: deleting a scanned `.js` file can *shrink* the
  output. `input.js` and `select.js` each had a `handleBlur`, which is the only
  reason `.blur` — and the ten `@property --tw-*` filter registrations it pulls
  in — were ever emitted.
- Phase 5 stepped on it twice, in comments both times. Explaining the query
  variant it uses cost **336 B** — the variant's name minus its `@` is also a
  Tailwind utility, and while the `@`-prefixed spelling in a class attribute is
  extracted whole and is harmless, the bare word in English is not. The rewrite
  of that comment — a note warning about *this exact trap*, which named
  "dropdown" and "list" to cite the example above — cost **8,590 B**.
- Phase 6 stepped on it three more times, for **3,765 B**: "the exclude list in
  css/app.css" (`.list`, 2,402 B) and "the stat abbreviation" (`.stat`,
  1,363 B), plus a repeat of the same "dropdown"/"list" sentence.

**A boundary rule worth knowing, found while bisecting those:** the punctuation
after the word decides it. `fixed list,` costs nothing and `their list and`
costs 2,402 B — a trailing comma keeps the word from being a candidate, a
trailing space does not. So the same word is safe or expensive depending on
where it falls in the sentence, which makes this impossible to eyeball. Every
one of these five was found by the selector diff, none changed a line of markup,
and none would have shown up in review. **Diff after editing a comment, not just
after editing markup.**

- Phases 7 and 8 leaked nothing. Phase 8 ran the mechanic in reverse instead:
  deleting `spell-slots-table.js` **removed** `.pl-0.5`, `.text-[0.71rem]` and
  `.tracking-[0.08em]`, the classes of a heading none of the four components ever
  rendered. Phase 6 predicted exactly that and named this phase.
- Phase 7 leaked nothing, which was the first clean phase since this was found.
  Worth recording as method rather than luck: the dangerous vocabulary is
  *whatever DaisyUI names that this build does not already emit*, and that set is
  checkable in one command against the previous selector snapshot. Words already
  in the file — `card`, `link`, `label`, `select`, `alert`, `modal` — are free to
  write; `list`, `stat`, `table`, `tab`, `menu` and the rest are not.

**So for every remaining phase: diff the emitted selectors, not the byte count,**
and check that anything new is something the markup actually uses. Rewording a
comment is usually the whole fix; `exclude:` is there for the cases where it is
not. Noted in `css/app.css` above the `@source` line as well.

---

## Open decisions

- **D4** — accept DaisyUI's faint field border, or override to 55%? Decide by
  looking at Phase 2.
- **D5** — how to fix `.validator-hint` under caramellatte (2.55:1). Three
  options above; recommend deciding in Phase 2 with a real error on screen.
- ~~**D6** — htmx fragments vs Alpine `x-for` for the two repeaters.~~ **Resolved
  in Phase 7: htmx fragment route.** `x-for` was never really in it — it puts the
  row in the page twice, once in templ and once in Alpine syntax, which is the
  lit-html duplication in a different dialect. The real contest was against a
  `<template>` clone, and htmx won on Phase 8's indexed card, not on Phase 7.
- ~~**Phase 8** — keep the indexed spell wire format (A) or move to
  order-based (B).~~ **Resolved: order-based.** Reasoning in Phase 8 above; the
  short version is that indices need a counter and a counter is client state.
