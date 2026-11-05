VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT)
BINARY   = epack-collector-sentry
EPACK_VERSION = $(shell go list -m -f '{{.Version}}' github.com/locktivity/epack)

.PHONY: build test lint clean build-all e2e trace-map

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/epack-collector-sentry

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

# Rewrites docs/schema/trace-map.json from the collectortest suites and the
# source, at the epack version go.mod pins, as CI checks it. Commit the result.
trace-map:
	go run -tags components github.com/locktivity/epack/cmd/epack@$(EPACK_VERSION) sdk map

e2e:
	go test -tags e2e -race -v -count=1 ./internal/sentry/

build-all:
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-amd64  ./cmd/epack-collector-sentry
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-arm64  ./cmd/epack-collector-sentry
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-darwin-amd64 ./cmd/epack-collector-sentry
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-darwin-arm64 ./cmd/epack-collector-sentry

clean:
	rm -f $(BINARY)
	rm -rf dist/
