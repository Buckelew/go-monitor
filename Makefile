include .env
export

run:
	go run cmd/monitor/main.go
.PHONY: run

refresh:
	./scripts/refresh.sh
.PHONY: refresh

migrate-create:
	migrate create -ext sql -dir migrations '$(word 2,$(MAKECMDGOALS))'
.PHONY: migrate-create

migrate-up:
	migrate -path migrations -database '$(POSTGRES_URL)' up
.PHONY: migrate-up

seed:
	psql $$POSTGRES_URL -f scripts/seed.sql
.PHONY: seed

bin-deps:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
.PHONY: bin-deps
