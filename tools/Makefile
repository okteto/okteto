COMMIT_SHA ?= $(shell git rev-parse --short HEAD)
BIN_DIR := ../bin

.DEFAULT_GOAL := build

.PHONY: build
build: remote supervisor clean-tool

.PHONY: remote
remote:
	CGO_ENABLED=0 go build -o $(BIN_DIR)/remote \
		-ldflags "-X main.CommitString=$(COMMIT_SHA)" \
		-tags "osusergo netgo static_build" \
		./remote/cmd/

.PHONY: supervisor
supervisor:
	CGO_ENABLED=0 go build -o $(BIN_DIR)/supervisor \
		-ldflags "-X main.CommitString=$(COMMIT_SHA)" \
		-tags "osusergo netgo static_build" \
		./supervisor/cmd/

.PHONY: clean-tool
clean-tool:
	CGO_ENABLED=0 go build -o $(BIN_DIR)/clean \
		-tags "osusergo netgo static_build" \
		./clean/

.PHONY: clean
clean:
	rm -f $(BIN_DIR)/remote $(BIN_DIR)/supervisor $(BIN_DIR)/clean

.PHONY: test
test:
	go test -p 4 -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: dep
dep:
	go mod tidy
