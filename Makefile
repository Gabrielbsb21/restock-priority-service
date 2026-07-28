.PHONY: build run test test-race lint docker-up docker-down

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

test:
	go test -v ./...

test-race:
	go test -race ./...

lint:
	go vet ./...

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v
