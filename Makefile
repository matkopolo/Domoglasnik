PYTHON := .venv/bin/python
PYTEST := .venv/bin/pytest
ETL := .venv/bin/real-estate-etl
PIPELINE_DIR := apps/data-pipeline
DATABASE_URL ?= postgresql://real_estate:real_estate@localhost:5432/real_estate
WORKERS ?= 6

.PHONY: setup db-up db-migrate db-down db-status lint test test-unit import-data import-data-verbose

setup:
	python3 -m venv .venv
	.venv/bin/python -m pip install --upgrade pip
	.venv/bin/pip install -e "$(PIPELINE_DIR)[dev]"

db-up:
	docker compose up -d --wait db
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/007_postgis.sql

db-migrate:
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U real_estate -d real_estate -f /docker-entrypoint-initdb.d/007_postgis.sql

db-down:
	docker compose down

db-status:
	docker compose ps

lint:
	cd $(PIPELINE_DIR) && ../../.venv/bin/ruff check .

test:
	cd $(PIPELINE_DIR) && TEST_DATABASE_URL=$(DATABASE_URL) ../../$(PYTEST) -q

test-unit:
	cd $(PIPELINE_DIR) && ../../$(PYTEST) -q -m "not integration"

import-data:
	$(ETL) refresh-all-data --workers $(WORKERS)

import-data-verbose:
	$(ETL) refresh-all-data --workers $(WORKERS) --verbose
