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
	GOOS=$(os) GOARCH=$(arch) go build -ldflags "-X github.com/okteto/cnd/cmd.VersionString=${VERSION_STRING}" -o "bin/cnd-$(os)-$(arch)" 

.PHONY: build
build:
	 go build -ldflags "-X github.com/okteto/cnd/cmd.VersionString=${VERSION_STRING}" -o ${BINDIR}/cnd