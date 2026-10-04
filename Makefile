.PHONY: db reset sqlc templ protocol css css-watch ts-check js js-test run check fmt fmt-check vet test

run: db templ sqlc protocol css js
	docker compose up --build

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

protocol:
	cd ./server && go generate ./...

css:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css

css-watch:
	./build/bin/tailwindcss -i ./server/css/app.css -o ./server/public/css/app.css --watch

ts-check:
	./node_modules/.bin/tsc -p ./server/js/tsconfig.json

js: ts-check
	./node_modules/.bin/esbuild ./server/js/journal-editor.js \
		--bundle --minify --format=esm --target=es2022 \
		--outfile=./server/public/static/journal-editor.js
	./node_modules/.bin/esbuild ./server/js/room/main.ts \
		--bundle --minify --format=esm --target=es2022 --sourcemap \
		--outfile=./server/public/static/room.js
	./node_modules/.bin/esbuild ./server/js/sound-preview.ts \
		--bundle --minify --format=esm --target=es2022 \
		--outfile=./server/public/static/sound-preview.js

js-test: ts-check
	node --test ./server/js/room/*.test.ts ./server/js/room/debug/*.test.ts ./server/js/room/model/*.test.ts ./server/js/room/modes/*.test.ts ./server/js/room/gl/*.test.ts ./server/js/room/render/*.test.ts ./server/js/room/render/stages/*.test.ts

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
