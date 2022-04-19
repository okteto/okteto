ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin
PLATFORMS := linux/amd64/okteto-Linux-x86_64 darwin/amd64/okteto-Darwin-x86_64 windows/amd64/okteto.exe linux/arm64/okteto-Linux-arm64 darwin/arm64/okteto-Darwin-arm64
BUILDCOMMAND := go build -trimpath -ldflags "-s -w -X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}" -tags "osusergo netgo static_build"
temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
label = $(word 3, $(temp))

UNAME := $(shell uname)
ifeq ($(UNAME), Darwin)
SHACOMMAND := shasum -a 256
else
SHACOMMAND := sha256sum
endif

.DEFAULT_GOAL := build

.PHONY: release
build-all: $(PLATFORMS)

$(PLATFORMS):
	GOOS=$(os) GOARCH=$(arch) CGO_ENABLED=0 $(BUILDCOMMAND) -o "bin/$(label)"
	$(SHACOMMAND) "bin/$(label)" > "bin/$(label).sha256"

.PHONY: latest
latest:
	echo ${VERSION_STRING} > bin/latest

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test -p 4 -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: integration
integration:
	go test github.com/okteto/okteto/integration -tags="common integration actions" --count=1 -v -timeout 1h

.PHONY: integration-actions
integration-actions:
	go test github.com/okteto/okteto/integration -tags="common actions" --count=1 -v -timeout 15m

.PHONY: build
build:
	$(BUILDCOMMAND) -o ${BINDIR}/okteto

.PHONY: build-integration
build-integration:
	go test github.com/okteto/okteto/integration -tags "common integration actions" -c -o ${BINDIR}/okteto-integration.test

.PHONY: dep
dep:
	go mod tidy
