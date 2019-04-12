.PHONY: push
push:
	docker push gcr.io/free-tier-219708/api

.PHONY: docker
docker:
	docker build -t gcr.io/free-tier-219708/api .

.PHONY: start
start:
	gin --port 8000 run main.go