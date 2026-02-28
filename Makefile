.PHONY: db sqlc templ client run

run: db templ sqlc client
	docker compose up --build

db:
	./build/migrate.sh

templ:
	templ generate

sqlc:
	sqlc generate

client:
	npm run build --prefix ./client
