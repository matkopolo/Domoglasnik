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
NODE_RUN := docker run --rm -v "$(CURDIR)/$(WEB_DIR):/app" -v $(WEB_NODE_MODULES):/app/node_modules -w /app $(NODE_IMAGE)

.PHONY: setup db-up db-migrate db-down db-status analytics-refresh models-refresh api-up api-down api-logs api-fmt api-vet api-test api-build web-setup web-up web-down web-logs web-lint web-test web-build lint pipeline-lint test pipeline-test test-unit import-data import-data-verbose

setup:
	python3 -m venv .venv
	.venv/bin/python -m pip install --upgrade pip
	.venv/bin/pip install -e "$(PIPELINE_DIR)[dev,ml]"

db-up:
	docker compose up -d --wait db
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/007_postgis.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/008_api_foundation.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/009_geographic_hierarchy.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/010_sale_package_classification.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/011_sale_facts.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/012_sale_outliers.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/013_api_read_only_role.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/014_rental_facts.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/015_rental_outliers.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/016_anomaly_signals.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/017_identity_and_preferences.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/018_valuation_models.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/019_market_indices.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/020_land_methodology.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/021_index_vintages.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/022_quality_observability.sql

db-migrate:
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/007_postgis.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/008_api_foundation.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/009_geographic_hierarchy.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/010_sale_package_classification.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/011_sale_facts.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/012_sale_outliers.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/013_api_read_only_role.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/014_rental_facts.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/015_rental_outliers.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/016_anomaly_signals.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/017_identity_and_preferences.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/018_valuation_models.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/019_market_indices.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/020_land_methodology.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/021_index_vintages.sql
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/022_quality_observability.sql

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

web-lint:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm run lint"

web-test:
	$(NODE_RUN) sh -lc "npm ci --prefer-offline >/dev/null && npm test"

web-build:
	docker compose build web

lint: pipeline-lint api-vet web-lint

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
