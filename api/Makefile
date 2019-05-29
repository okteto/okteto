BINDIR    := $(CURDIR)/bin

.PHONY: build
build:
	go build -o bin/okteto cmd/okteto/main.go

.PHONY: push
push:
	docker push gcr.io/free-tier-219708/api

.PHONY: docker
docker:
	DOCKER_BUILDKIT=1 docker build -t gcr.io/free-tier-219708/api .

.PHONY: start
start:
	cd cmd/okteto; gin --port 8080 run