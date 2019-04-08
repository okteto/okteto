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
	GO111MODULE=on GOOS=$(os) GOARCH=$(arch) go build -ldflags "-X main.VersionString=${VERSION_STRING}" -o "bin/okteto-$(os)-$(arch)" 
	sha256sum "bin/okteto-$(os)-$(arch)" > "bin/okteto-$(os)-$(arch).sha256"  

.PHONY: build
build:
	go get -d -v ./...
	GO111MODULE=on go build -ldflags "-X main.VersionString=${VERSION_STRING}" -o ${BINDIR}/okteto

.PHONY: test
test:
	GO111MODULE=on go test ./...

.PHONY: get
get:
	go get -d -v ./...

.PHONY: dep
dep:
	GO111MODULE=on go mod tidy