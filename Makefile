ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin
PLATFORMS := linux/amd64/okteto-Linux-x86_64 darwin/amd64/okteto-Darwin-x86_64 windows/amd64/okteto.exe linux/arm64/okteto-Linux-arm64
BUILDCOMMAND := go build -ldflags "-s -w -X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}" -tags "osusergo netgo static_build"
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
	 go test -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: integration
integration:
	 go test github.com/okteto/okteto/integration -tags=integration --count=1 -v

.PHONY: build
build:
	 $(BUILDCOMMAND) -o ${BINDIR}/okteto

.PHONY: build-bin-image
build-bin-image:
	 okteto build -t okteto/bin:1.1.22 -f images/bin/Dockerfile images/bin

.PHONY: dep
dep:
	go mod tidy