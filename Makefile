.PHONY: db sqlc templ css css-watch run

run: db templ sqlc css
	docker compose up --build

db:
	./build/migrate.sh

templ:
	templ generate

sqlc:
	sqlc generate

# Tailwind is a standalone binary (no Node). It is gitignored -- see the
# pinned download command in PLAN.md if build/bin/tailwindcss is missing.
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
