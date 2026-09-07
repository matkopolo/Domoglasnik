PYTEST := .venv/bin/pytest
PIPELINE_DIR := apps/data-pipeline
PIPELINE_RUN := docker compose run --rm pipeline-worker
DATABASE_URL ?= postgresql://real_estate:real_estate@localhost:5432/real_estate
WORKERS ?= 6
GO_IMAGE ?= golang:1.26.5-bookworm
API_DIR := apps/real-estate-api
API_TEST_DATABASE_URL ?= postgresql://real_estate:real_estate@host.docker.internal:5432/real_estate
GO_RUN := docker run --rm -v "$(CURDIR)/$(API_DIR):/src" -v real_estate_go_mod_cache:/go/pkg/mod -v real_estate_go_build_cache:/root/.cache/go-build -w /src $(GO_IMAGE)
WEB_DIR := apps/web
NODE_IMAGE ?= node:22-bookworm-slim
WEB_NODE_MODULES := real_estate_web_node_modules_debian
NODE_RUN := docker run --rm -v "$(CURDIR):/repo" -v $(WEB_NODE_MODULES):/repo/$(WEB_DIR)/node_modules -w /repo/$(WEB_DIR) $(NODE_IMAGE)
.PHONY: setup start refresh-system reset-and-repopulate db-up db-migrate db-down db-status analytics-refresh models-refresh api-up api-down api-logs api-fmt api-vet api-test api-build web-setup web-up web-down web-logs web-generate-api-types web-contract-drift web-lint web-typecheck web-test web-build lint pipeline-lint test pipeline-test test-unit import-data import-data-verbose

setup:
	python3 -m venv .venv
	.venv/bin/python -m pip install --upgrade pip
	.venv/bin/pip install -e "$(PIPELINE_DIR)[dev,ml]"

start:
	docker compose up -d --build --wait

refresh-system: db-up
	$(PIPELINE_RUN) refresh-system --workers $(WORKERS) --output-dir /data/raw

reset-and-repopulate: db-up
	$(PIPELINE_RUN) reset-and-repopulate --confirm-reset --workers $(WORKERS) --output-dir /data/raw

db-up:
	docker compose up -d --wait db
	$(MAKE) db-migrate

db-migrate:
	docker compose run --rm database-migrations

analytics-refresh: db-up
	$(PIPELINE_RUN) refresh-analytics

models-refresh: db-up
	$(PIPELINE_RUN) refresh-models --workers $(WORKERS)

db-down:
	docker compose down

db-status:
	docker compose ps

api-up: db-up
	docker compose up -d --build --wait real-estate-api

api-down:
	docker compose stop real-estate-api

api-logs:
	docker compose logs -f real-estate-api

api-fmt:
	$(GO_RUN) gofmt -w .

api-vet:
	$(GO_RUN) go vet ./...

api-test:
	docker run --rm -v "$(CURDIR)/$(API_DIR):/src" -v real_estate_go_mod_cache:/go/pkg/mod -v real_estate_go_build_cache:/root/.cache/go-build -e TEST_DATABASE_URL=$(API_TEST_DATABASE_URL) -w /src $(GO_IMAGE) go test -race ./...

api-build:
	docker compose build real-estate-api

web-setup:
	$(NODE_RUN) npm ci

web-up: api-up
	docker compose up -d --build --wait web

web-down:
	docker compose stop web

web-logs:
	docker compose logs -f web

web-generate-api-types:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm run generate:api-types"

web-contract-drift: web-generate-api-types
	git diff --exit-code -- $(WEB_DIR)/lib/generated/api-schema.d.ts

web-lint:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm run lint"

web-typecheck:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm run typecheck"

web-test:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm test"

web-build:
	docker compose build web

lint: pipeline-lint api-vet web-lint web-typecheck

pipeline-lint:
	cd $(PIPELINE_DIR) && ../../.venv/bin/ruff check .

test: pipeline-test api-test web-test

pipeline-test:
	cd $(PIPELINE_DIR) && TEST_DATABASE_URL=$(DATABASE_URL) ../../$(PYTEST) -q

test-unit:
	cd $(PIPELINE_DIR) && ../../$(PYTEST) -q -m "not integration"

import-data: db-up
	$(PIPELINE_RUN) refresh-all-data --workers $(WORKERS) --output-dir /data/raw

import-data-verbose: db-up
	$(PIPELINE_RUN) refresh-all-data --workers $(WORKERS) --output-dir /data/raw --verbose

# Production helpers never delete a namespace, Cluster, PVC or backup.
K8S := sh infrastructure/kubernetes/scripts/operator.sh
.PHONY: k8s-render-single k8s-render-ha k8s-db-apply-single k8s-db-apply-ha k8s-db-wait k8s-migrate k8s-apps-apply-single k8s-apps-apply-ha k8s-status k8s-db-status k8s-logs-api k8s-logs-pipeline
k8s-render-single:
	$(K8S) render single-node database
	$(K8S) render single-node apps
k8s-render-ha:
	$(K8S) render ha database
	$(K8S) render ha apps
k8s-db-apply-single:
	$(K8S) apply single-node database
k8s-db-apply-ha:
	$(K8S) apply ha database
k8s-db-wait:
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate wait --for=condition=Ready cluster/real-estate-db --timeout=900s
k8s-migrate:
	$(K8S) job single-node migrations
k8s-apps-apply-single:
	$(K8S) apply single-node apps
k8s-apps-apply-ha:
	$(K8S) apply ha apps
k8s-status:
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate get all
k8s-db-status:
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate get cluster/real-estate-db -o wide
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate get pods -l cnpg.io/cluster=real-estate-db -o wide
k8s-logs-api:
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate logs deployment/real-estate-api -f
k8s-logs-pipeline:
	kubectl --context "$(KUBE_CONTEXT)" -n real-estate logs deployment/pipeline-worker -f
