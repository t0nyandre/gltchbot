.PHONY: build build-bot build-api run-bot run-api sqlc docker-up docker-down docker-logs tidy vet

SQLC := $(shell which sqlc 2>/dev/null || echo $(HOME)/go/bin/sqlc)

## Build both binaries
build: build-bot build-api

build-bot:
	go build -ldflags="-s -w" -o ./bin/bot ./cmd/bot

build-api:
	go build -ldflags="-s -w" -o ./bin/api ./cmd/api

## Run locally (requires .env and a running postgres)
run-bot:
	go run ./cmd/bot

run-api:
	go run ./cmd/api

## Generate sqlc code from SQL queries
sqlc:
	$(SQLC) generate

## Docker Compose helpers
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

## Go maintenance
tidy:
	go mod tidy

vet:
	go vet ./...
