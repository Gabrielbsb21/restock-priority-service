.PHONY: build run test test-race cover fmt lint tidy check migrate migrate-down migrate-status docker-up docker-down

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test -race ./...

# -coverpkg attributes coverage across packages, so code exercised by another
# package's tests is not reported as untested.
cover:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

fmt:
	gofmt -w .

lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt required for:"; echo "$$out"; exit 1; fi
	go vet ./...

tidy:
	go mod tidy

# check runs the quality gates from docs/engineering/go-guidelines.md.
check: lint test test-race

migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down -v
