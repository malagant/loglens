.PHONY: build run test lint snapshot release-dry tidy demo demo-gif demo-cast

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

# ---- Demo re-rendering ----------------------------------------------------
# Requires `vhs` (Charm) for the GIF and `asciinema` + `expect` for the cast.
# Both targets put a freshly built `loglens` on PATH before recording so they
# reflect the working tree, not whatever `loglens` happens to be installed.

demo: demo-gif demo-cast

demo-gif: build
	PATH="$(CURDIR)/bin:$$PATH" vhs docs/demo.tape

demo-cast: build
	@command -v asciinema >/dev/null || { echo "asciinema not on PATH" >&2; exit 1; }
	@command -v expect    >/dev/null || { echo "expect not on PATH"    >&2; exit 1; }
	PATH="$(CURDIR)/bin:$$PATH" expect -c '\
	  set ::env(TERM) "xterm-256color"; \
	  spawn -noecho asciinema rec --rows 28 --cols 110 --overwrite "-c" "docs/cast.sh" "docs/demo.cast"; \
	  stty rows 28 columns 110 < $$spawn_out(slave,name); \
	  expect eof'
