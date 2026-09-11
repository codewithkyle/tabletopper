.PHONY: db reset sqlc templ protocol css css-watch ts-check js js-test run check fmt fmt-check vet test

run: db templ sqlc protocol css js
	docker compose up --build

# Nuke the local database and come back up from nothing. `docker compose down -v`
# drops the db_data volume MySQL keeps its files in, so the next `make db`
# re-applies every migration against an empty schema rather than against the
# rows that were there.
#
# THIS DELETES EVERY LOCAL ROW, your own account included -- which is usually
# the point. Signing in again writes a fresh users row, and a fresh row has
# never answered the welcome dialog, so the onboarding flow runs.
#
# IT DOES NOT TOUCH R2, and cannot usefully be made to. The bucket is remote and
# may be shared, and the assets rows that name its objects are in the database
# this just dropped -- so whatever it held is now unreachable, and the hourly
# sweeper cannot collect it either, because the sweep finds objects by reading
# those same rows. Clear the bucket by hand if the orphans matter.
#
# The prompt is here because `reset` and `run` are one keystroke apart and one
# of them is not recoverable. `make reset FORCE=1` skips it. Reading from a
# closed stdin fails the comparison, so a non-interactive run aborts rather
# than proceeding.
reset:
	@if [ -z "$(FORCE)" ]; then \
		printf 'This deletes every row in the local database. Type "nuke" to continue: '; \
		read answer; \
		[ "$$answer" = "nuke" ] || { echo "Aborted; nothing was deleted."; exit 1; }; \
	fi
	docker compose down -v
	$(MAKE)

db:
	./build/migrate.sh

templ:
	templ generate

sqlc:
	sqlc generate

# The virtual tabletop's wire protocol, in TypeScript, from the Go types that
# define it. The server validates every message, so its structs are the
# authority on what a message is; a second hand-written set of types on the
# client would agree right up until somebody changed one of them, and the
# disagreement would be found at a table rather than in a build.
#
# It writes exactly one file: server/js/room/protocol.ts, from
# server/internal/room. The file is committed, and
# TestProtocolTypesAreCurrent regenerates into a buffer and fails when the two
# disagree -- so a stale copy breaks `make check` rather than a browser, and
# this target is the fix rather than the guard.
#
# The room bundle imports it: see the js target below, whose type check is what
# turns a protocol change the client has not followed into a build failure.
protocol:
	cd ./server && go generate ./...

# Tailwind is a standalone binary (no Node). It is gitignored; if
# build/bin/tailwindcss is missing, fetch the pinned version with:
#
#   mkdir -p build/bin
#   curl -sL -o build/bin/tailwindcss \
#     https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.3/tailwindcss-linux-x64
#   chmod +x build/bin/tailwindcss
#
# It builds exactly one file: server/public/css/app.css, from the entry point
# server/css/app.css (Tailwind + the vendored DaisyUI plugin and themes).
# Every other stylesheet in server/public/css is hand-written and served as
# authored -- no source dir, no build step. They use native CSS nesting, which
# browsers run directly.
#
# Nothing here is minified on purpose. Cloudflare Brotli-compresses CSS at the
# edge, which is worth ~6x what minifying is; minifying first would buy under a
# kilobyte and cost readable diffs on app.css, which is committed.
css:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css

css-watch:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --watch

# The TypeScript type check, which is a separate target because two things
# want it: the bundle below, and `make check`.
#
# esbuild STRIPS TYPES WITHOUT CHECKING THEM, which is what makes it fast and
# what makes this necessary. Without this a type error would bundle cleanly and
# fail in a browser; with it, it fails the build the way a Go compile error
# does. The config is server/js/tsconfig.json and it emits nothing.
ts-check:
	./node_modules/.bin/tsc -p ./server/js/tsconfig.json

# The two browser bundles. node_modules is gitignored; if it is missing,
# install the pinned tree with:
#
#   npm ci
#
# Node 24 or newer, which package.json declares.
#
# THE FIRST IS THE JOURNAL EDITOR, which is Tiptap and is npm-only -- it is why
# this repo has a package.json at all. server/js/journal-editor.js becomes
# server/public/static/journal-editor.js, an ES module the journal entry page
# loads and no other page does.
#
# THE SECOND IS THE ROOM, which is TypeScript: server/js/room/main.ts and
# everything it imports become server/public/static/room.js. The room page
# loads it with the server build on the URL, so a deploy changes the URL and
# the one-hour cache on /static/ cannot answer the reload with the script that
# was there before it.
#
# BOTH OUTPUTS ARE MINIFIED and app.css is not, which is not an inconsistency:
# app.css is committed and read in diffs, while these are build artifacts whose
# source is what gets reviewed. The room bundle carries a sourcemap for exactly
# that reason -- it is our own code, so a stack trace from a table has to lead
# back to the TypeScript it came from.
js: ts-check
	./node_modules/.bin/esbuild ./server/js/journal-editor.js \
		--bundle --minify --format=esm --target=es2022 \
		--outfile=./server/public/static/journal-editor.js
	./node_modules/.bin/esbuild ./server/js/room/main.ts \
		--bundle --minify --format=esm --target=es2022 --sourcemap \
		--outfile=./server/public/static/room.js

# The client reducer, replayed against the golden fixtures the Go tests
# generate. Node runs the .ts files directly -- erasableSyntaxOnly in the
# tsconfig is what keeps them to the syntax it can strip -- and node:test is in
# the standard library, so there is no test framework here to install.
#
# A CHANGE TO internal/room/reduce.go FAILS THIS. The fixtures are regenerated
# by `go test ./internal/room -update`, and the TypeScript port has to follow
# them before the build is green again. That is the whole reason a reducer
# exists in Go: the server never reduces its own events.
js-test: ts-check
	node --test ./server/js/room/*.test.ts ./server/js/room/model/*.test.ts ./server/js/room/render/*.test.ts

# Formatting is enforced, not suggested: `make check` is what CI and a
# pre-commit hook should run. fmt-check lists every offending file before it
# fails so the fix is one `make fmt` away. templ fmt has no dry-run flag, so
# each file is formatted to stdout and diffed against itself.
check: fmt-check vet test js-test

fmt:
	gofmt -w ./server
	templ fmt ./server/templ

fmt-check:
	@unformatted="$$(gofmt -l ./server)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt: needs formatting:"; echo "$$unformatted"; exit 1; fi
	@status=0; \
	for f in $$(find ./server/templ -name '*.templ'); do \
		if ! templ fmt -stdout "$$f" 2>/dev/null | diff -q - "$$f" >/dev/null; then echo "templ fmt: needs formatting: $$f"; status=1; fi; \
	done; exit $$status

vet:
	cd ./server && go vet ./...

test:
	cd ./server && go test -race ./...
