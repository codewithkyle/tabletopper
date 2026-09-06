# Map tiling

Maps become a tile pyramid built on the server at upload. The renderer then fetches
only the tiles the viewport covers at the zoom it is at, and never decodes a
108-megapixel image on anybody's laptop.

## Why server-side

The alternative was tiling in the browser. It doesn't work, for a reason that isn't
about cost: to slice a 12000x9000 image in the browser you must first download and
decode all of it -- ~20 MB over the wire and 432 MB of RGBA resident. Tiling *after*
that has already paid every price tiling exists to avoid. It solves `MAX_TEXTURE_SIZE`
(8192 on plenty of hardware, which is why the old SPA clamps to 8000 in
`client/src/pages/tabletop-page/tabletop-component/table-canvas/table-canvas.ts:355`)
and nothing else. And a VTT multiplies it: a DM and five players is six full downloads
and six full decodes of the same map, every session, with the least capable device at
the table deciding whether the map opens at all.

Server-side, it is paid once, ever.

The storage cost that made this a question is not the deciding factor. A pyramid is a
geometric series -- 1 + 1/4 + 1/16 + ... -- so it converges at 1.33x the base level;
call it 1.5x with per-tile compression overhead, and ~2.5x if the original is retained
alongside it, which this plan does retain. At R2's $0.015/GB-month, a thousand
12000x9000 maps is well under a dollar a month.

The costs that are real are upload latency (hundreds of PUTs, which is why this is a
background job and not a handler), encode time (the pyramid is ~144 megapixels of WebP,
which is tens of seconds on one core) and decode memory (half a gigabyte per in-flight
tiling job, which is why exactly one runs at a time).

## Decisions

**Tile size is 512, not 256.** 256 is a Leaflet default from when browsers opened six
connections per host. At 512 a 12000x9000 map is 584 objects instead of 2276 -- 4x
fewer PUTs on upload, 4x fewer requests on a pan, 4x less per-tile header and
compression overhead -- and 512 is nowhere near any GPU limit. It is stored per asset
rather than baked in, so changing it later re-tiles from the retained original instead
of requiring a re-upload.

**Level 0 is native resolution; each level up is halved, rounding up.** The row stores
native `width` and `height`, and every other number follows from them:

```
levelW(z) = (width  + (1<<z) - 1) >> z        // ceil(width / 2^z)
levelH(z) = (height + (1<<z) - 1) >> z
tilesX(z) = (levelW(z) + tileSize - 1) / tileSize
tilesY(z) = (levelH(z) + tileSize - 1) / tileSize
max_zoom  = smallest z at which levelW(z) <= tileSize and levelH(z) <= tileSize
```

Ceil, not a bare right-shift. A bare shift floors, and flooring 9000 five times gives
281 where the pixels say 282: a level one row short of its last pixel. Three things
compute these numbers -- the tiler, the validator in Phase 5 and the renderer -- and
the formulas are written once, here, because two of them agreeing and the third
flooring renders the wrong tile with no error anywhere.

The renderer picks its level in one line:

```
z = clamp(floor(log2(1 / cameraScale)), 0, maxZoom)
```

This is the inverse of the slippy-map convention (where z increases with detail), and
it is worth being deliberate about because mixing the two silently renders the wrong
level. Written down here because the renderer does not exist yet to hold the comment.

For a 12000x9000 map at 512:

| Level | Pixels | Tiles |
| --- | --- | --- |
| 0 | 12000x9000 | 24x18 = 432 |
| 1 | 6000x4500 | 12x9 = 108 |
| 2 | 3000x2250 | 6x5 = 30 |
| 3 | 1500x1125 | 3x3 = 9 |
| 4 | 750x563 | 2x2 = 4 |
| 5 | 375x282 | 1x1 = 1 |

`max_zoom` = 5, 584 objects, ~30 MB.

**Every pyramid is a generation, and a generation is never modified.** Each tiling job
mints a ULID and writes its tiles under it. The row names the generation that is
serving in `tile_gen`, and every tile URL carries it. A replaced map, a retried job
and a re-tile at a new tile size each produce a new generation at new keys and new
URLs; the old one keeps serving until the new one is complete, and is deleted then.

This is what makes the immutable caching in Phase 5 true rather than asserted.
Without it a replacement lands at the same asset ID, the same z/x/y and therefore the
same URLs, and a player who opened the old map keeps its tiles for a year. It also
removes an ordering hazard: nothing has to delete the old pyramid before queueing the
new one, and a map that is being replaced never has a window with no tiles at all.
Two pyramids exist for the minute a rebuild takes; that is the cost.

**The worker owns `width`, `height` and `max_zoom`, not the upload request.** The
request's header pass reads the dimensions as stored. The tiler decodes with the EXIF
orientation applied, as the upload path already does for avatars and for the same
reason. A phone photo of a hand-drawn map with a rotation tag would otherwise get a row
that says 9000x12000 and a pyramid that is 12000x9000, and the renderer would compute
the wrong grid from the row. The worker writes all three at completion, in the same
statement as `tile_gen`, so the row's description of the pyramid and the pyramid
itself change together.

**The original is retained and never served.** It is the source for re-tiling if the
tile size or encoder ever changes, and re-tiling from a lossy pyramid would compound.
It is stored as uploaded -- not re-encoded -- which is also what lets the request hand
off to the worker without decoding anything. `file_path` holds its key. It cannot be
served through `streamImage` as things stand: that function hardcodes `Content-Type:
image/webp` (`server/internal/controllers/assets.go`), and the original is whatever
PNG or JPEG arrived. Nothing should route to it.

**The worker reads the original back out of R2 rather than being handed the decoded
image.** A channel of decoded images would die on restart and bound concurrency by
memory pressure instead of by intent. Reading from R2 keeps the existing invariant
intact: the row is the ledger for what the bucket holds, and the worker's input is an
object the ledger already names.

**Deferred.** Whether tiles are proxied through the app or served from an R2 custom
domain at the edge. A map load is now hundreds of requests through `streamImage`
instead of one, which is a real load question, but it is answerable after there is a
renderer making those requests. It is also not only a load question: an R2 custom
domain is public, so moving there changes the auth model to signed URLs or a Worker in
front of the bucket. R2's free egress matters only then; proxied, a tile costs the
app's bandwidth, which is fine pre-release. Phase 5 keeps them proxied and
auth-gated, like every other image today.

## Key layout

The current `storage.MapKeys` is flat -- `users/{u}/maps/{id}` and
`users/{u}/maps/preview-{id}`. A pyramid needs a prefix per asset and a prefix per
generation, so maps move to a directory:

```
users/{userID}/maps/{assetID}/original
users/{userID}/maps/{assetID}/{gen}/preview
users/{userID}/maps/{assetID}/{gen}/z{z}/{x}_{y}.webp
```

The preview lives under its generation, so it is built from the pyramid it belongs to
and goes when that pyramid goes. `preview_path` on the row points at it and moves at
completion; `serveImage` revalidates it against `updated_at` as it does today, so the
card picks up the new one on its next paint.

Two prefixes are what deletion needs: the generation's, when a newer one supersedes
it, and the asset's, when the map is deleted. Each is one list plus one batch delete
rather than 584 calls.

**Existing map rows do not fit this layout and there is no backfill in this plan.**
`yet-another-rewrite` is pre-release; the assumption is the bucket's maps get cleared
by hand and re-uploaded. If that assumption is wrong, stop and add a backfill phase --
a half-migrated bucket where some rows are flat and some are prefixed is worse than
either.

## Phase 0 -- make a large upload physically possible

None of the rest matters until a 12000x9000 file can reach the process. Four things
currently stop it, and only one is the size cap everyone thinks of.

1. **`ReadTimeout: 5 * time.Second` in `server/main.go` is the hard blocker.** It
   covers reading the entire request body, not the headers, so it is a bandwidth
   floor: 8 MiB in 5s already demands ~13 Mbps up, and a 100 MB PNG is hopeless. Move
   the server to `ReadHeaderTimeout: 5 * time.Second` with `ReadTimeout` unset.
   Leaving `ReadTimeout` global and merely larger would hand every other route the
   same slowloris window.

2. **`WriteTimeout: 10 * time.Second` blocks it just as hard, and less obviously.**
   net/http sets the write deadline the moment the request headers are read, as an
   absolute time: request start plus ten seconds. A sixty-second upload reads to
   completion and then cannot write its response. So the upload handlers extend
   *both* deadlines, `SetReadDeadline` and `SetWriteDeadline`, through
   `http.NewResponseController(w)`, and nothing else changes: the global values stay
   where they are for every other route.

3. **`maxUploadBytes` and `maxUploadPixels`** (8 MiB / 40 MP) become something like
   128 MiB / 150 MP. 12000x9000 is 108 MP, so the current pixel cap rejects the
   motivating case outright.

4. **`r.ParseMultipartForm(maxUploadBytes)` passes the cap as the in-memory limit.**
   That is harmless at 8 MiB and a 128 MB heap allocation at the new cap. The argument
   is how much to keep in RAM before spilling to a temp file; it should stay small
   (32 MiB) and independent of the cap, while `http.MaxBytesReader` keeps enforcing the
   cap. Above the threshold the body spills to disk, so a 100 MB upload is a 100 MB
   file in the container's `/tmp` for the life of the request. The image has to have
   the room.

`readImageUpload` splits in two. Today it validates and then decodes. Maps need only
the first half -- the header pass, the format check and the pixel cap, then the raw
bytes -- and avatars keep the whole thing. The header-first pass is exactly right and
gets more valuable here: it refuses an oversized canvas before a decoder allocates
anything. The dimensions it reads are checked against the cap and not stored; see the
EXIF decision above.

## Phase 1 -- schema and keys

Migration adding to `assets`, NULL for every non-map type, the way `journal_id` and
`detached_at` were added:

| Column | Purpose |
| --- | --- |
| `width`, `height` | native pixels of the serving pyramid, written by the worker at completion; NULL until then |
| `tile_size` | 512 today; stored so it can change |
| `max_zoom` | highest level of the serving pyramid |
| `tile_gen` | ULID of the serving generation; NULL until the first completes. In the URL of every tile |
| `tile_state` | ENUM `pending`, `working`, `ready`, `failed` -- the job, not the pyramid |
| `tile_attempts` | retry counter, so a poison upload stops |
| `tile_lease` | ULID of the generation being built; the claim token |
| `tile_leased_at` | when it was claimed; the restart-recovery clock |
| `tiled_at` | when the serving pyramid completed |

The job columns and the pyramid columns are separate on purpose. A replaced map is
`working` while its old generation is still serving, and the tile route reads
`tile_gen`, `width`, `height`, `tile_size` and `max_zoom` without looking at
`tile_state` at all.

Index on `(tile_state, created_at)` -- the worker's claim query is the only read that
isn't already covered, and it wants the oldest pending row.

**The column is `tile_state`, not `tile_status`.** `status` is on the list in
CLAUDE.md of bare words that have leaked a DaisyUI component family into
`server/public/css/app.css`, and generated sqlc identifiers reach `.templ` files.
`state` costs nothing and cannot.

Then `storage.MapKeys` splits into `MapOriginalKey`, `MapPreviewKey`, `MapTileKey`,
`MapPrefix` and `MapGenerationPrefix`, and `storage.Client` gains `DeletePrefix`
(`ListObjectsV2` + `DeleteMany`, paged at 1000).

## Phase 2 -- the tiler

New `server/internal/tiler`, pure image manipulation, no R2 and no database, so it is
testable on synthetic images.

```go
// Build decodes src and hands every tile of the pyramid to emit, level 0
// first, as decoded pixels. Encoding is the caller's.
func Build(src io.Reader, tileSize int, emit func(Tile) error) (Result, error)
```

- Decode once with `imaging.Decode(..., imaging.AutoOrientation(true))` -- the EXIF
  handling that is already the reason the upload path uses imaging rather than
  `image.Decode`. `Result` carries the oriented width and height, which is what the
  row stores.
- Level 0 crops from the source. **Each subsequent level halves the level above, not
  the source.** Going back to the source each time is a full-resolution pass per
  level, six times over.
- **The halving is a 2x2 box average, not Lanczos.** At exactly 2:1 the box is the
  right filter: each destination pixel is the mean of the four source pixels it
  covers, nothing aliases and nothing rings. Lanczos at that ratio is a worse filter
  at a far higher price, and imaging's implementation is the price that matters: any
  resize that changes both axes allocates an intermediate of destination width by
  source height, 216 MB for the first halving of a 12000x9000 map. Use
  `draw.ApproxBiLinear` from `golang.org/x/image/draw`, already an indirect
  dependency: at 2:1 it is exactly the four-pixel blend, it has fast paths for every
  type Go's decoders produce (NRGBA, RGBA, Gray and every YCbCr subsampling), and it
  allocates nothing but the destination. The destination is `*image.RGBA`, which is
  the one type `encodeWebP`'s encoder takes without a conversion copy. An odd level
  dimension is what the ceil in Decisions means: the last column or row averages the
  pixels that exist.
- Edge tiles are the source's remainder, not padded to square. Padding would put
  transparent or black seams along two edges of every map, and the renderer already
  knows the native dimensions and can size the last row and column itself.
- `emit` is a callback rather than a returned slice so nothing accumulates 584 tiles
  in memory before the first one is written. It hands over decoded pixels, 1 MB per
  tile, and blocks when the caller's pool is full, which is the backpressure that
  keeps the tiler from running ahead of the encoders.

Peak memory is the decoded source plus level 1 plus the tiles in flight. The source is
released once level 0's tiles are emitted and level 1 exists; from level 2 on the
numbers are small.

| Source, 12000x9000 | Decoded | Level 1 | Peak, one job |
| --- | --- | --- | --- |
| PNG (RGBA) | 432 MB | 108 MB | ~560 MB |
| JPEG (YCbCr 4:2:0) | 162 MB | 108 MB | ~290 MB |

So the ceiling is `5 x maxUploadPixels` bytes, and the box is sized for that.

Tests: a 1x1 source (max_zoom 0, one tile), a source exactly one tile wide, a source
one pixel over a tile boundary (the off-by-one that produces a 1px-wide edge tile), a
non-square source that reaches 1x1 at different levels per axis, an odd-sized level
(the ceil), and a JPEG with an orientation tag (the pyramid's dimensions are the
rotated ones).

## Phase 3 -- the worker

New `server/internal/tiling`, started from `run()` in `server/main.go` next to
`sweep.JournalImages` and `sweep.ExpiredShares`, hanging off the same signal context.
It follows the sweeper shape those two established: run once at startup, then on a
ticker, because a process that has just come up should not wait for the interval to do
the work it was down for.

**Exactly one job at a time.** A 108 MP PNG is 432 MB decoded before anything else is
allocated; two concurrent uploads on a small box is an OOM. Serialized, the memory
ceiling is `5 x maxUploadPixels` bytes and the box can be sized for it. This is also
what makes pure Go viable here -- see the note below.

One pass, after recovery:

1. Claim the oldest `pending` row with a fresh ULID as the lease:
   `UPDATE assets SET tile_state='working', tile_lease=?, tile_leased_at=NOW() WHERE
   type='map' AND tile_state='pending' ORDER BY created_at LIMIT 1`, then
   `SELECT ... WHERE tile_lease=?`. MySQL has no RETURNING, so the token is how the
   row is found again, and the conditional update is the lock: two processes cannot
   claim the same map. The lease ULID is also the generation the job writes under, so
   one value names both the claim and the prefix it produces.
2. `Storage.Get` the original.
3. `tiler.Build`, with `emit` handing each tile to a bounded pool (~16) that encodes
   it with the existing `encodeWebP` and PUTs it. Encode is where the time goes --
   about 144 megapixels of WebP at quality 75 through libwebp, tens of seconds on one
   core -- and the PUTs are latency. One pool doing both keeps the cores busy while
   the round trips overlap; a pool that only PUTs leaves the encode serial.
4. Write the preview from the smallest level rather than from the source. `square(src,
   256)` on a full-resolution image is a second full-resolution resample for a
   thumbnail; the top of the pyramid is already about that size.
5. Complete, conditionally: `UPDATE ... SET tile_state='ready', tile_gen=tile_lease,
   width, height, max_zoom, tile_size, preview_path, tiled_at=NOW(), tile_lease=NULL,
   tile_leased_at=NULL WHERE id=? AND tile_lease=? AND tile_state='working'`. Zero rows
   affected means the row was re-queued underneath the job -- a replacement arrived
   mid-tile -- and the pyramid just built describes an original that is gone:
   `DeletePrefix` the generation and move on; the pending row is picked up next pass.
   Otherwise `DeletePrefix` the previous generation, if there was one.

On failure: `DeletePrefix` the generation, `tile_state='failed'`, increment
`tile_attempts`, clear the lease. A partial pyramid under a ULID nobody will use again
is exactly an orphan, which is why it goes now rather than being left for a retry to
overwrite -- retries write a new generation.

**Restart recovery.** A row left `working` by a killed process is stranded forever
with nothing to notice. Each pass first takes `working` rows whose `tile_leased_at` is
older than a lease window (15 minutes), deletes the lease's generation prefix, and
returns them to `pending`; `failed` rows under a few attempts go back to `pending`
likewise. So a transient R2 outage retries and a genuinely broken image stops. A job
has to finish well inside the window, and it does by an order of magnitude; a second
process reclaiming a live job would delete its tiles from under it.

The ledger holds: every prefix in the bucket is named by a row, as `tile_gen` while
serving or as `tile_lease` while being built, and recovery is what keeps the second
half true across a crash.

### On pure Go versus libvips

Full decode of the source is unavoidable in pure Go, so the half gigabyte is real.
Serialized, that is acceptable, and it is what this plan ships: no new dependency
beyond promoting `x/image` from indirect, and the tiler interface is narrow enough to
swap.

libvips is the escape hatch if that ceiling becomes a problem, and it is cheaper here
than it looks -- `server/Dockerfile` already sets `CGO_ENABLED=1` and already installs
`libwebp-dev` in the builder and `libwebp` in the runtime, so adopting it is adding
`vips-dev` and `vips` to two existing `apk add` lines. It does shrink-on-load and
pyramid generation in bounded memory, which would let the job run concurrently and drop
the peak by an order of magnitude. Not now, but the reason it is easy is worth
recording.

## Phase 4 -- the upload path

`UploadMap` gets simpler, not more complex. It stops re-encoding the full image
entirely -- that is `encodeWebP(src)` on 108 MP inside a request handler -- and stops
building the preview, which moves to the worker.

1. The header-only read for validation: format, byte cap, pixel cap. Nothing is
   decoded and no dimensions are kept.
2. Insert the row `pending` with `file_path` set to the original's key and
   `tile_size`. The pyramid columns stay NULL.
3. PUT the original bytes as received.
4. Render `MapCard`.

The existing row-first-object-second rollback and `discardMap` carry over unchanged.

`ReplaceMap` does the ownership check and the header read, PUTs the original at its
fixed key -- overwriting -- and then sets `tile_state='pending'`, `tile_attempts=0`
and the file name. Nothing is deleted in the request. The old generation keeps
serving through `tile_gen` until the worker completes the new one and deletes it; a
job that was mid-flight is discarded by the completion conditional in Phase 3. The
existing note in `ReplaceMap` -- "replacing overwrites the existing keys, so nothing
can be orphaned" -- stays true of the original and stops being true of tiles, which
are never overwritten, only superseded. It needs to say so.

`DeleteMap` swaps `DeleteMapObjects` for `DeletePrefix` on the asset prefix, keeping
objects-first-row-last. That takes the original and every generation, in-flight
included.

Retry is `POST /assets/maps/{id}/tiles`: owner-scoped, `RequireSession`, sets a
`failed` row back to `pending` with `tile_attempts=0`, and renders the card. A
mutation with a resource URL, answering with the row it changed.

## Phase 5 -- serving tiles

```
GET /assets/maps/{id}/tiles/{gen}/{z}/{x}_{y}.webp
```

Not under `/fragment/` -- it returns an image, not partial HTML. `RequireSessionOr404`,
matching the two routes under `/assets/images/`, and unscoped to the owner for the
reason `GetImage` already is: every player at the table has to be able to fetch the
map.

Validate everything against the row: `gen` parses as a ULID and equals `tile_gen`;
`z <= max_zoom`; `x < tilesX(z)` and `y < tilesY(z)` by the formulas in Decisions. A
row with a NULL `tile_gen` has no tiles and answers 404 to everything. Any failure is
a bare 404. Never `http.NotFound` -- it writes a page-shaped body into what the
renderer is treating as an image.

**Tiles are immutable and cached as such.** This is the one place the caching policy
departs from the rest of the app, and deliberately. An avatar or a preview can be
replaced at its URL, which is why `serveImage` sets `private, no-cache` and revalidates
against an `updated_at` ETag. A tile cannot: the generation is in the URL, a replaced
map serves a new generation at new URLs, and a re-tile at a different tile size does
the same. The bytes at a tile URL never change. So `Cache-Control: private,
max-age=31536000, immutable`, and a player panning back over ground they have already
seen makes zero requests.

The renderer needs `width`, `height`, `tile_size`, `max_zoom` and `tile_gen` to
compute tile URLs. Serve them as attributes on the element that mounts the canvas, not
as a manifest fetch -- the page render already has the row.

## Phase 6 -- the processing state

A card for a map that has never been tiled has no preview and cannot be dragged onto a
table. It polls until it is ready:

```
GET /fragment/assets/maps/{id}/card
```

Registered behind `auth.Fragment(...)` in `server/routes.go`, rendering the same
`pages.MapCard` the page render uses. While the row is `pending` or `working`, the card
carries `hx-trigger="load delay:2s"` / `hx-swap="outerHTML"` pointed at itself; a card
whose job is done carries neither, so the poll stops by virtue of what came back.

The card is three independent questions, not one three-way switch, because a
replacement can be in flight over a pyramid that still serves:

- **Usable** -- `tile_gen` is set. Preview shown, draggable. Independent of the job.
- **Polling** -- `tile_state` is `pending` or `working`. A usable card polls too, and
  stays usable while it does.
- **Retry** -- `tile_state` is `failed`. Shows the button that POSTs to
  `/assets/maps/{id}/tiles`.

`MapCard` currently renders the preview `<img>` behind `if m.PreviewPath.Valid`. Those
three become methods on the page-data type in `server/templ/pages/assets.go`, which is
where reasoning about them can be written down at all.

**Two mechanical rules apply to this phase specifically.** No comment of any kind goes
into `assets.templ` -- not `//`, not HTML, not a doc block. And after the markup
changes, check what the stylesheet gained:

```sh
cp server/public/css/app.css /tmp/app.before.css && make css
diff <(grep -oE '^\s*\.[^ {,:]+' /tmp/app.before.css | sort -u) \
     <(grep -oE '^\s*\.[^ {,:]+' server/public/css/app.css | sort -u)
```

Every added selector should be one that can be pointed at in the new markup.

## Phase 7 -- the renderer

Out of scope. The VTT does not exist in `./server` yet. What this plan owes it is a
pyramid, five values on the asset row, and an immutable tile URL, all of which the
phases above deliver and none of which depend on how the renderer works.

What it inherits, written down now so the pyramid's shape is not second-guessed later:

- **Map-pixel space is native at every level.** Tile `(z, x, y)` covers the native
  rectangle starting at `(x * tileSize << z, y * tileSize << z)` with size
  `(tileW << z, tileH << z)`. Tokens, the grid, fog and pointer math never see `z`;
  they keep the single-quad coordinate model the old renderer had.
- **The parent of `(z, x, y)` is `(z+1, x>>1, y>>1)`.** Draw it, already cached,
  while the children load. The level convention above is what makes that a shift.
- **The tile cache is a texture array.** Layers of `tileSize` square, LRU by layer
  index, one instanced draw per level. A 4K viewport at native zoom shows about forty
  tiles; 128 layers is 128 MB and generous. An edge tile occupies part of a layer and
  its quad's UV maximum is `tileW / tileSize`.
- **The fetch path is the standard one.** `fetch` to `blob` to `createImageBitmap`,
  then `texImage2D` in WebGL2 or `copyExternalImageToTexture` in WebGPU. Same-origin
  cookie auth works unchanged; see Deferred for what changes if tiles move to the
  edge.
- **Level selection replaces mipmaps.** No `generateMipmap`. `floor` in the formula
  gives sharper output and more tiles; `round` gives fewer tiles and slight softness.
  Renderer's call, no server impact.
- **The old fog composite is the one shader that does not port.** It samples the map
  and the mask at one UV, which assumes one map texture. Draw the tiles first, then
  one map-sized fog quad blended over them. Cleaner anyway, and the mask can stay low
  resolution.
- **Tiles have no gutter.** Bilinear sampling at fractional zoom clamps at a tile edge
  instead of blending into the neighbour. Leaflet and MapLibre ship without gutters
  and it is rarely visible. If it shows on hard grid lines, re-tile from the retained
  original with a one-pixel overlap -- a new tile size, a new generation, no
  re-upload. This is the case the retained original exists for.
- **WebGPU adds nothing.** Same ImageBitmap path, same array texture. Compressed GPU
  formats later would be a re-tile from the original, not a re-upload.

Nothing carries over from `client/` -- it clamps to 8000 and uploads one texture,
which is the approach this replaces.

## Order

Phases 0-1 land together; neither is useful alone and both are prerequisites for
everything else. 2 is independently testable and can be written in parallel. 3-4 land
together -- a worker with nothing queuing to it and a queue with no worker are each
half a feature. 5 and 6 are independent of each other.
