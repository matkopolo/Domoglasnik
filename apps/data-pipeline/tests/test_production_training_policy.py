from unittest.mock import MagicMock

import pytest

from real_estate_etl.operations import (
    RefreshAlreadyRunning,
    maintenance_lock,
    model_training_enabled,
)


def test_compose_keeps_automatic_training(monkeypatch):
    monkeypatch.delenv("MODEL_REFRESH_POLICY", raising=False)
    assert model_training_enabled()


def test_production_requires_explicit_training(monkeypatch):
    monkeypatch.setenv("MODEL_REFRESH_POLICY", "manual")
    assert not model_training_enabled()


def test_invalid_policy_fails_closed(monkeypatch):
    monkeypatch.setenv("MODEL_REFRESH_POLICY", "typo")
    with pytest.raises(ValueError, match="MODEL_REFRESH_POLICY"):
        model_training_enabled()


def test_manual_maintenance_releases_lock_after_failure():
    connection = MagicMock()
    connection.execute.return_value.fetchone.return_value = (True,)
    with pytest.raises(ValueError, match="operation failed"), maintenance_lock(connection):
        raise ValueError("operation failed")
    assert "pg_advisory_unlock" in connection.execute.call_args.args[0]
    connection.rollback.assert_called_once()


def test_manual_maintenance_refuses_concurrent_refresh():
    connection = MagicMock()
    connection.execute.return_value.fetchone.return_value = (False,)
    with pytest.raises(RefreshAlreadyRunning), maintenance_lock(connection):
        pytest.fail("Concurrent maintenance must not execute")
