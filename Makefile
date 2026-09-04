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
# Two kinds of CSS are built with it:
#   server/css/app.css      -- the Tailwind + DaisyUI entry point
#   server/css/app/*.css    -- this app's own stylesheets, hand-written with
#                              native CSS nesting. Formerly SCSS built by
#                              cssmonster in client/; the same binary compiles
#                              the nesting away, so no Node and no Sass.
# Everything else in server/public/css is hand-maintained with no source:
# the component CSS whose SCSS was lost, plus normalize.css and tokens.css.
APP_CSS_SRC := $(wildcard server/css/app/*.css)
APP_CSS_OUT := $(patsubst server/css/app/%.css,server/public/css/%.css,$(APP_CSS_SRC))

css: $(APP_CSS_OUT)
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --minify

server/public/css/%.css: server/css/app/%.css
	@./build/bin/tailwindcss -i $< -o $@ --minify

css-watch:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --watch
