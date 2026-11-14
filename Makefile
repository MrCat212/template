.PHONY: migrate run generate test test-integration lint

migrate:
	@set -a; . ./.env; set +a; go tool goose -dir ./migrations postgres "$$DATABASE_URL" up

run:
	@set -a; . ./.env; set +a; go run ./cmd/trip-service

generate:
	go tool oapi-codegen -config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/usecase/... ./internal/http/trip/...

test:
	go test -race -cover ./...

test-integration:
	@set -a; . ./.env; set +a; go test -tags=integration -race -cover ./...

lint:
	golangci-lint run ./...