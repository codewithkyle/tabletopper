.PHONY: db sqlc templ run

run: db templ sqlc
	docker compose up --build

db:
	./build/migrate.sh

templ:
	templ generate

sqlc:
	sqlc generate
