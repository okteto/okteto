.PHONY: prod
prod: check build-api build-frontend tag-prod

.PHONY: tag-prod
tag-prod:
	docker tag api gcr.io/okteto-prod/api:${TAG}
	docker tag frontend gcr.io/okteto-prod/frontend:${TAG}
	git tag "cloud-${TAG}"
	yq w -i chart/okteto/Chart.yaml appVersion "${TAG}"

.PHONY: push-prod
push-prod: 
	docker push gcr.io/okteto-prod/api:${TAG}
	docker push gcr.io/okteto-prod/frontend:${TAG}

.PHONY: update-prod
update-prod:
	helm upgrade --tls -f /keybase/team/riberaproject/private/okteto-cloud/override-prod.yaml okteto chart/okteto
	git push --tag origin

.PHONY: upgrade-prod-cli
upgrade-prod-cli:
	git tag ${TAG}
	git push --tag origin

.PHONY: check
check:
	git branch | grep \* | cut -d ' ' -f2 | grep master

.PHONY: build-api
build-api: 
	docker build -t api api

.PHONY: build-frontend
build-frontend: 
	docker build -t frontend frontend
