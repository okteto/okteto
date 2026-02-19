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
	golangci-lint run -v --timeout 5m

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix --timeout 5m

.PHONY: test
test:
	OKTETO_DEPLOYABLE="" OKTETO_FORCE_REMOTE="" OKTETO_DEPLOY_REMOTE="" go test -p 4 -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: integration
integration:
	go test github.com/okteto/okteto/integration/... -tags="common integration actions" --count=1 -v -timeout 1h

.PHONY: integration-actions
integration-actions:
	go test github.com/okteto/okteto/integration/actions -tags="actions" --count=1 -v -timeout 10m

.PHONY: integration-build
integration-build:
	go test github.com/okteto/okteto/integration/build -tags="integration" --count=1 -v -timeout 10m

.PHONY: integration-deploy
integration-deploy:
	go test github.com/okteto/okteto/integration/deploy -tags="integration" --count=1 -v -timeout 20m

.PHONY: integration-okteto
integration-okteto:
	go test github.com/okteto/okteto/integration/okteto -tags="integration" --count=1 -v -timeout 30m

.PHONY: integration-up
integration-up:
	go test github.com/okteto/okteto/integration/up -tags="integration" --count=1 -v -timeout 45m

.PHONY: integration-okteto-test
integration-okteto-test:
	go test github.com/okteto/okteto/integration/test -tags="integration" --count=1 -v -timeout 45m

.PHONY: integration-gateway
integration-gateway:
	@if [ "$(ARGS)" = "gateway" ]; then \
		go test github.com/okteto/okteto/integration/gateway -tags="integration" --count=1 -v -timeout 30m -run="Gateway"; \
	elif [ "$(ARGS)" = "ingress" ]; then \
		go test github.com/okteto/okteto/integration/gateway -tags="integration" --count=1 -v -timeout 30m -run="Ingress"; \
	else \
		go test github.com/okteto/okteto/integration/gateway -tags="integration" --count=1 -v -timeout 30m; \
	fi

.PHONY: build
build:
	$(BUILDCOMMAND) -o ${BINDIR}/okteto

.PHONY: build-integration
build-integration:
	go test github.com/okteto/okteto/integration -tags "common integration actions" -c -o ${BINDIR}/okteto-integration.test

.PHONY: dep
dep:
	go mod tidy

.PHONY: codecov
codecov:
	go test -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: generate-schema
generate-schema:
	go run . generate-schema -o schema.json
