GOCMD := go
GOTEST := $(GOCMD) test ./...
GOLINT := golangci-lint run
VERSION ?= $(shell git describe --tags --always)

.PHONY: all build test lint vet

all: vet lint test build ## everything a CI would run

build:
	$(GOCMD) build -ldflags "-X main.version=$(VERSION)" ./cmd/ror

test:
	$(GOTEST) -v

lint:
	$(GOLINT)

vet:
	$(GOCMD) vet ./...
