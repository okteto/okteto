ifeq ($(strip $(VERSION_STRING)),)
VERSION_STRING := $(shell git rev-parse --short HEAD)
endif

BINDIR    := $(CURDIR)/bin

build:
	go build -ldflags "-X github.com/okteto/cnd/cmd.VersionString=${VERSION_STRING}" -o ${BINDIR}/cnd