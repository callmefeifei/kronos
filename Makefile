VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -X github.com/pstrr/kronos/cmd/commands.Version=$(VERSION) \
           -X github.com/pstrr/kronos/cmd/commands.GitCommit=$(GIT_COMMIT) \
           -X github.com/pstrr/kronos/cmd/commands.BuildTime=$(BUILD_TIME)

.PHONY: build dev clean

build:
	@if [ -d web/node_modules ]; then cd web && npm run build; fi
	go build -ldflags "$(LDFLAGS)" -o bin/kronos ./cmd/kronos

dev:
	go run -ldflags "$(LDFLAGS)" ./cmd/kronos serve

clean:
	rm -rf bin/
