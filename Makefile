ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin
PLATFORMS := linux/amd64/okteto-Linux-x86_64/osusergo*netgo*static_build darwin/amd64/okteto-Darwin-x86_64/osusergo*netgo*static_build windows/amd64/okteto.exe/osusergo*static_build linux/arm64/okteto-Linux-arm64/osusergo*netgo*static_build darwin/arm64/okteto-Darwin-arm64/osusergo*netgo*static_build
BUILDCOMMAND := go build -trimpath -ldflags "-s -w -X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}"
temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
label = $(word 3, $(temp))
tags = $(subst *, ,$(word 4, $(temp)))

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
	GOOS=$(os) GOARCH=$(arch) CGO_ENABLED=0 $(BUILDCOMMAND) -tags "$(tags)" -o "bin/$(label)"
	$(SHACOMMAND) "bin/$(label)" > "bin/$(label).sha256"

.PHONY: latest
latest:
	echo ${VERSION_STRING} > bin/latest

.PHONY: lint
lint:
	pre-commit run --all-files
	golangci-lint run

.PHONY: test
test:
	go test -p 4 -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: e2e
e2e:
	go test github.com/okteto/okteto/e2e/... -tags="common e2e actions" --count=1 -v -timeout 1h

.PHONY: e2e-actions
e2e-actions:
	go test github.com/okteto/okteto/e2e/actions -tags="actions" --count=1 -v -timeout 10m

.PHONY: e2e-build
e2e-build:
	go test github.com/okteto/okteto/e2e/build -tags="e2e" --count=1 -v -timeout 10m

.PHONY: e2e-deploy
e2e-deploy:
	go test github.com/okteto/okteto/e2e/deploy -tags="e2e" --count=1 -v -timeout 20m

.PHONY: e2e-okteto
e2e-okteto:
	go test github.com/okteto/okteto/e2e/okteto -tags="e2e" --count=1 -v -timeout 30m

.PHONY: e2e-up
e2e-up:
	go test github.com/okteto/okteto/e2e/up -tags="e2e" --count=1 -v -timeout 45m

.PHONY: e2e-deprecated
e2e-deprecated:
	go test github.com/okteto/okteto/e2e/deprecated/push -tags="e2e" --count=1 -v -timeout 15m && go test github.com/okteto/okteto/e2e/deprecated/stack -tags="e2e" --count=1 -v -timeout 15m

.PHONY: build
build:
	$(BUILDCOMMAND) -o ${BINDIR}/okteto

.PHONY: build-e2e
build-e2e:
	go test github.com/okteto/okteto/e2e -tags "common e2e actions" -c -o ${BINDIR}/okteto-e2e.test

.PHONY: dep
dep:
	go mod tidy

.PHONY: codecov
codecov:
	go test -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
