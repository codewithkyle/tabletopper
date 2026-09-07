# Asset manager expansion

Four kinds of asset behind one sub-nav: Maps (done), Tokens, Avatars, Music.

## Decisions locked

- **Character portraits move to their own enum member (`character`).** `avatar`
  becomes the NPC library the GM manages. Without this the Avatars page lists
  every player's portrait, and Delete on that page drops the row
  `characters.asset_id` points at -- there are no foreign keys anywhere in this
  schema, so nothing would notice until the portrait rendered broken.
- **Music and ambience are one thing.** A long track that plays to completion,
  loops, and keeps looping until the GM stops it or picks another. No category
  column, no per-row loop flag: looping is what every track does, and one plays
  at a time.
- **Audio is served by presigned R2 URL**, not proxied. And uploaded by
  presigned URL too -- see Phase 2.
- Phase order is 0 -> 1 -> 2 -> 3 as below (avatars and tokens before music).

## Phase 0 -- the sub-nav and three shells -- LANDED 2026-09-07

No data work. Ships on its own.

`characterTabLink` in `templ/pages/character-tabs.templ` is already generic
(href, name, current) and only its name is character-flavoured. Rename it to
`tabLink` and add `assetTabs(active string)` beside it, rendering the four
links in `appBar()` where the character editor renders `characterTabs`. The
underline row is right here rather than the spell tabs' pill row: asset kinds
are sections of one manager, which is what the character tabs mean too.

Routes -- literals, not `GET /assets/{kind}`. `/assets/images/{id}` already
occupies a sibling segment, and routes.go's own reasoning prefers a named route
over a parameter with a handful of legal values.

    GET /assets            -> redirect /assets/maps   (unchanged)
    GET /assets/maps                                  (unchanged)
    GET /assets/tokens     -> empty state
    GET /assets/avatars    -> empty state
    GET /assets/music      -> empty state

The empty states follow the manual's (`monsterCards` in `monsters.templ`).

Also, while the name control is about to be copied onto three more pages: the
map name input carries no `maxlength` and `ReplaceMapName` bounds nothing, so a
paste longer than `VARCHAR(255)` is a 500. Fix it here.

## Phase 1 -- the migration, then Avatars and Tokens -- LANDED 2026-09-07

### Migration

Append `character` to the type ENUM (instant metadata change), then move the
existing rows. Down reverses it rows-first, which is the ordering
20260906280000 documents: narrowing an ENUM that still holds rows in the member
being dropped does not fail, it silently rewrites them.

    -- up
    ALTER TABLE assets MODIFY `type` ENUM(
        'map','avatar','token','music','journal','monster','character'
    ) NOT NULL DEFAULT 'map';
    UPDATE assets SET `type` = 'character' WHERE `type` = 'avatar';

Every existing `avatar` row is a character portrait -- `InsertAvatar` is the
only statement that writes that member -- so the UPDATE is unconditional.

The down migration merges library avatars back into character portraits. That
is inherent and there is no way to tell them apart afterwards; it is written
here so nobody discovers it by running one.

### Objects already in the bucket are not moved

Reads never rebuild a key: `serveImage` uses `asset.FilePath`, which the
migration does not touch. So portraits stored under `users/{u}/avatars/{id}`
keep serving after their type changes.

Two paths *do* rebuild it, and both must be changed to read `FilePath` from the
row instead: the replace branch of `UploadCharacterAvatar`, and
`discardAvatar`. With those two fixed the migration is safe against live data
and needs no object copy.

New key builders in `internal/storage/keys.go`, flat like `MonsterImageKey`:

    CharacterPortraitKey  users/{u}/portraits/{id}   (new portraits only)
    AvatarKey             users/{u}/avatars/{id}     (now the library)
    TokenKey              users/{u}/tokens/{id}
    MusicKey              users/{u}/music/{id}

### Queries

In `server/sql/assets.sql`:

- `InsertAvatar` -> `InsertCharacterPortrait`, writing `type = 'character'`.
- `GetImage`: add `'character'` to the IN list. Not `'music'` -- that route
  answers `image/webp` and audio never goes through it.
- New `GetLibraryAssets` (owner + type) and `GetLibraryAsset` (id + owner +
  type). One pair serves all three library pages; the kind is validated against
  a Go allowlist before it reaches either, the way the monster action `kind`
  segment is.
- New `UpdateAssetDimensions` (id + owner + width + height), for tokens.

### Avatars and Tokens share one handler path

Both are: header pass -> decode -> resize -> WebP -> row-then-object -> card.
Write it once, parameterised by a kind descriptor (enum member, key builder,
target size, fit-or-fill), with four thin route registrations per kind:

    GET    /assets/{kind}
    POST   /assets/{kind}
    POST   /assets/{kind}/{id}        (replace)
    PATCH  /assets/{kind}/{id}/name
    DELETE /assets/{kind}/{id}

Maps stay on their own handlers because of tiling. Music does not use this path
at all.

**The one difference between the two kinds is the resize.** `images.Square`
uses `imaging.Fill`, which centre-crops -- that turns a longboat into a square
of hull. Avatars keep Fill (a portrait is a thumbnail and cropping is correct).
Tokens need `imaging.Fit` into a 512 box with aspect preserved. Add
`images.Fit` beside `images.Square`.

Transparency survives the existing encoder: chai2010/webp dispatches an RGBA
image to `EncodeRGBA` on the lossy path, which encodes the alpha plane. Cut-out
tokens need no lossless special case.

**Tokens write `width` and `height`.** Those columns already exist -- the
tiling migration added them, NULL for everything but maps -- and are documented
as the oriented pixels. Storing them means the VTT can place a token at its
correct aspect without fetching and decoding it first.

### The card

Tokens and avatars are the map card minus the tiling overlay. One shared card
component with the poll/retry block rendered only for maps, rather than three
near-copies.

## Phase 2 -- Music -- NEXT

The only kind that is not an image. Every helper from `openImageUpload` down is
image-only, and every serving route ends in `streamImage`, which hardcodes
`Content-Type: image/webp` and ignores `Range`.

### Upload goes browser -> R2 directly

A 2-hour track is 115-175 MB. Through the existing multipart path that is a
175 MB heap allocation, a 175 MB spill to the container's `/tmp`, and a
10-minute read deadline that amounts to demanding 2.3 Mbps -- a slow connection
times out mid-body -- followed by the Go process re-uploading all of it to R2.

So:

1. `POST /assets/music` carries the filename and the declared byte size. No
   body. It writes the assets row first -- the ledger rule everything else here
   obeys -- naming the key the object will land at, and answers with a
   presigned PUT URL.
2. The server refuses to sign anything over the cap, and **signs
   `Content-Length` and `Content-Type` into the URL**. That is what keeps the
   cap enforceable when the bytes never pass through Go: R2 rejects a body that
   is not the size the signature names.
3. The browser PUTs to R2 with progress from an XHR upload listener.
4. `POST /assets/music/{id}/confirm` -- the server HEADs the object, then does a
   ranged GET of the first 64 bytes and sniffs the magic. On anything it does
   not recognise, or a size over the cap, it deletes the object and rolls the
   row back exactly as `discardMap` does. On success it answers with the card.

`internal/storage` gains `PresignPut`, `PresignGet` and a `Head`/ranged read.
`s3.NewPresignClient` comes from the aws-sdk-go-v2 already in go.mod; no new
dependency.

**Prerequisite outside the code:** the R2 bucket needs a CORS policy allowing
PUT and GET from the app origin, with `Content-Type` and `Range` in the allowed
headers. Nothing in this phase works until that is set in the Cloudflare
dashboard.

### Sniffing

There is no decoder registry for audio the way `image.DecodeConfig` is one for
images, so the allowlist is a magic-byte check -- the honest analogue of the
header pass, and the reason the browser's declared Content-Type is not trusted:

    ID3 / 0xFF Ex frame sync    mp3
    OggS                        ogg (vorbis, opus)
    ftyp at offset 4            m4a / mp4 (aac)
    1A 45 DF A3                 EBML -- webm/mkv
    fLaC                        flac
    RIFF....WAVE                wav

The EBML entry is not optional if these tracks come off YouTube: yt-dlp's
default audio output is frequently opus in a WebM container, which is not an
Ogg page and would be refused by an ogg-only check.

The signed `Content-Type` is what R2 stores and what it serves back on the
presigned GET, so playback needs no content-type column.

### Playback

`GET /assets/music/{id}/url` mints a presigned GET and answers JSON. It is a
GET returning JSON, not partial HTML, so it does not go under `/fragment/`.

**Expiry has to outlast the track.** A URL that dies mid-session 403s the next
range request the browser makes -- a seek, or a buffer refill after a stall --
and the music stops. Mint with a wide expiry (6h) and have the player re-mint
once on a 403 before giving up.

No preview, no waveform, no duration. `preview_path`, `width` and `height` stay
NULL. Duration would mean parsing frame headers or granule positions
server-side; the browser gets it free from `loadedmetadata`, but reporting it
back is a whole extra mutation route for a number nothing needs yet.

### Cap

256 MiB. Covers 2 hours at 256 kbps with room, and is a number the presigned
PUT can actually enforce.

## Phase 3 -- Search

One fragment route with the kind validated against the four members:

    GET /fragment/assets/list?kind=&q=

Both parameters checked before anything is queried, and the term bounded by the
name column's width -- the shape `/fragment/monster/list` already uses. The
handler switches on kind to pick the card.

## Every phase that touches server/templ

No comments of any kind in a `.templ` file. Reasoning about a component goes in
the Go that renders or reads it -- the handler in `internal/controllers`, or the
page-data type in `templ/pages/*.go`.

And after each change, diff what the stylesheet gained:

    cp server/public/css/app.css /tmp/app.before.css && make css
    diff <(grep -oE '^\s*\.[^ {,:]+' /tmp/app.before.css | sort -u) \
         <(grep -oE '^\s*\.[^ {,:]+' server/public/css/app.css | sort -u)

Every added selector should be one you can point at in the markup just written.
