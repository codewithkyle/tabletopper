.PHONY: db sqlc templ css css-watch js run check fmt fmt-check vet test

run: db templ sqlc css js
	docker compose up --build

db:
	./build/migrate.sh

templ:
	templ generate

sqlc:
	sqlc generate

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

# The journal editor is Tiptap, which is npm-only, so this one target is the
# whole reason the repo has a package.json. node_modules is gitignored; if it is
# missing, install the pinned tree with:
#
#   npm ci
#
# Node 24 or newer, which package.json declares. It bundles exactly one entry
# point: server/js/journal-editor.js becomes
# server/public/static/journal-editor.js, an ES module the journal entry page
# loads and no other page does.
#
# THE OUTPUT IS MINIFIED and app.css is not, which is not an inconsistency:
# app.css is committed and read in diffs, while this is 400 KB of vendored
# third-party code nobody reads either way. Brotli at the edge does the
# compression that matters in both cases.
js:
	./node_modules/.bin/esbuild ./server/js/journal-editor.js \
		--bundle --minify --format=esm --target=es2022 \
		--outfile=./server/public/static/journal-editor.js

# Formatting is enforced, not suggested: `make check` is what CI and a
# pre-commit hook should run. fmt-check lists every offending file before it
# fails so the fix is one `make fmt` away. templ fmt has no dry-run flag, so
# each file is formatted to stdout and diffed against itself.
check: fmt-check vet test

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
