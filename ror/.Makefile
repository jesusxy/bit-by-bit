GOCMD := go
GOTEST := $(GOCMD) test ./...
GOLINT := golangci-lint run
VERSION ?= $(shell git describe --tags --always)

.PHONY: all build test lint
all: build

build:
	$(GOCMD) build -ldflags "-X main.version=$(VERSION)" ./cmd/ror

test:
	$(GOTEST) -v

lint:
	$(GOLINT)
