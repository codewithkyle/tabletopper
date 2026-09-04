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
css:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --minify

css-watch:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --watch
