ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin
PLATFORMS := linux/amd64/Linux-x86_64 darwin/amd64/Darwin-x86_64 windows/amd64/Windows-x86_64

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
label = $(word 3, $(temp))

.DEFAULT_GOAL := build

.PHONY: release
build-all: $(PLATFORMS)

$(PLATFORMS):
	GOOS=$(os) GOARCH=$(arch) go build -ldflags "-X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}" -o "bin/okteto-$(label)" 
	sha256sum "bin/okteto-$(label)" > "bin/okteto-$(label).sha256"  

.PHONY: test
test:
	 go test ./...

.PHONY: integration
integration:
	 go test github.com/okteto/okteto/integration -tags=integration -v

.PHONY: build
build:
	 go build -ldflags "-X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}" -o ${BINDIR}/okteto

.PHONY: dep
dep:
	GO111MODULE=on go mod tidy