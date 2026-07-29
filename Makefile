PYTHON := .venv/bin/python
PYTEST := .venv/bin/pytest
ETL := .venv/bin/real-estate-etl
PIPELINE_DIR := apps/data-pipeline
DATABASE_URL ?= postgresql://real_estate:real_estate@localhost:5432/real_estate
WORKERS ?= 6
GO_IMAGE ?= golang:1.26.5-bookworm
API_DIR := apps/api
API_TEST_DATABASE_URL ?= postgresql://real_estate:real_estate@host.docker.internal:5432/real_estate
GO_RUN := docker run --rm -v "$(CURDIR)/$(API_DIR):/src" -v real_estate_go_mod_cache:/go/pkg/mod -v real_estate_go_build_cache:/root/.cache/go-build -w /src $(GO_IMAGE)
WEB_DIR := apps/web
NODE_IMAGE ?= node:22-bookworm-slim
WEB_NODE_MODULES := real_estate_web_node_modules_debian
NODE_RUN := docker run --rm -v "$(CURDIR):/repo" -v $(WEB_NODE_MODULES):/repo/$(WEB_DIR)/node_modules -w /repo/$(WEB_DIR) $(NODE_IMAGE)
MIGRATION_FILES := $(notdir $(wildcard database/migrations/0[0][7-9]_*.sql database/migrations/01*.sql database/migrations/02*.sql))

.PHONY: setup db-up db-migrate db-down db-status analytics-refresh models-refresh api-up api-down api-logs api-fmt api-vet api-test api-build web-setup web-up web-down web-logs web-generate-api-types web-contract-drift web-lint web-typecheck web-test web-build lint pipeline-lint test pipeline-test test-unit import-data import-data-verbose

setup:
	python3 -m venv .venv
	.venv/bin/python -m pip install --upgrade pip
	.venv/bin/pip install -e "$(PIPELINE_DIR)[dev,ml]"

db-up:
	docker compose up -d --wait db
	$(MAKE) db-migrate

db-migrate:
	@set -e; for migration in $(MIGRATION_FILES); do \
		echo "Applying $$migration"; \
		docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f "/docker-entrypoint-initdb.d/$$migration"; \
	done

analytics-refresh: db-up
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_sale_facts();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_land_transaction_facts();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_sale_outliers();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_rental_facts();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_rental_outliers();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_anomaly_signals();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_near_duplicate_candidates();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_missingness_profiles();"
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -c "SELECT analytics.refresh_repeat_sales_indices();"

models-refresh: db-up
	$(ETL) refresh-models --workers $(WORKERS)

db-down:
	docker compose down

db-status:
	docker compose ps

api-up: db-up
	docker compose up -d --build --wait api

api-down:
	docker compose stop api

api-logs:
	docker compose logs -f api

api-fmt:
	$(GO_RUN) gofmt -w .

api-vet:
	$(GO_RUN) go vet ./...

api-test:
	docker run --rm -v "$(CURDIR)/$(API_DIR):/src" -v real_estate_go_mod_cache:/go/pkg/mod -v real_estate_go_build_cache:/root/.cache/go-build -e TEST_DATABASE_URL=$(API_TEST_DATABASE_URL) -w /src $(GO_IMAGE) go test -race ./...

api-build:
	docker compose build api

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

import-data:
	$(ETL) refresh-all-data --workers $(WORKERS)

import-data-verbose:
	$(ETL) refresh-all-data --workers $(WORKERS) --verbose
