.PHONY: build run test lint snapshot release-dry tidy

build:
	go build -trimpath -ldflags "-s -w" -o bin/loglens ./cmd/loglens

run:
	go run ./cmd/loglens

test:
	go test ./...

lint:
	golangci-lint run

snapshot:
	goreleaser release --snapshot --clean --skip=publish,sign

release-dry:
	goreleaser release --snapshot --clean

tidy:
	go mod tidy
