ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin
PLATFORMS := linux/amd64 darwin/amd64 windows/amd64

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))

.DEFAULT_GOAL := build

.PHONY: release
build-all: $(PLATFORMS)

$(PLATFORMS):
	GO111MODULE=on GOOS=$(os) GOARCH=$(arch) go build -ldflags "-X github.com/cloudnativedevelopment/cnd/pkg/config.VersionString=${VERSION_STRING}" -o "bin/cnd-$(os)-$(arch)" 
	sha256sum "bin/cnd-$(os)-$(arch)" > "bin/cnd-$(os)-$(arch).sha256"  

.PHONY: build
build:
	 GO111MODULE=on go build -ldflags "-X github.com/cloudnativedevelopment/cnd/pkg/config.VersionString=${VERSION_STRING}" -o ${BINDIR}/cnd

.PHONY: dep
dep:
	GO111MODULE=on go mod tidy