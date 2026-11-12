.PHONY: migrate run generate test

migrate:
	@set -a; . ./.env; set +a; go tool goose -dir ./migrations postgres "$$DATABASE_URL" up

run:
	@set -a; . ./.env; set +a; go run ./cmd/trip-service

generate:
	go tool oapi-codegen -config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml

test:
	@set -a; . ./.env; set +a; go test -race ./...