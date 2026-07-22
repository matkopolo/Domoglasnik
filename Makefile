PYTHON := .venv/bin/python
PYTEST := .venv/bin/pytest
PIPELINE_DIR := apps/data-pipeline
PIPELINE_ENV := PYTHONPATH=$(PIPELINE_DIR)
DATABASE_URL ?= postgresql://real_estate:real_estate@localhost:5432/real_estate
WORKERS ?= 6

.PHONY: setup db-up db-down db-status test test-unit import-data import-data-verbose

setup:
	python3 -m venv .venv
	.venv/bin/pip install -r $(PIPELINE_DIR)/requirements-dev.txt

db-up:
	docker compose up -d db

db-down:
	docker compose down

db-status:
	docker compose ps

test:
	cd $(PIPELINE_DIR) && TEST_DATABASE_URL=$(DATABASE_URL) ../../$(PYTEST) -q

test-unit:
	cd $(PIPELINE_DIR) && ../../$(PYTEST) -q -m "not integration"

import-data:
	$(PIPELINE_ENV) $(PYTHON) -m real_estate_etl.cli refresh-all-data --workers $(WORKERS)

import-data-verbose:
	$(PIPELINE_ENV) $(PYTHON) -m real_estate_etl.cli refresh-all-data --workers $(WORKERS) --verbose
