.PHONY: build build-bot build-api run-bot run-api sqlc docker-up docker-down docker-logs docker-dev-up docker-dev-down docker-dev-logs tidy vet

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

## Docker Compose dev helpers (builds from source, wipes volumes on down)
docker-dev-up:
	docker compose --env-file .env.dev -f docker-compose.dev.yml up -d --build

docker-dev-down:
	docker compose --env-file .env.dev -f docker-compose.dev.yml down -v

docker-dev-logs:
	docker compose --env-file .env.dev -f docker-compose.dev.yml logs -f

## Go maintenance
tidy:
	go mod tidy

vet:
	go vet ./...
