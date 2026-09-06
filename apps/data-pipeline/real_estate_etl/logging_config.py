"""Consistent stdout logging for CLI and container execution."""

from __future__ import annotations

import logging
import os
import sys

DEFAULT_LOG_LEVEL = "INFO"


class _PipelineFormatter(logging.Formatter):
    """Render planned multi-line sections without a prefix on only their first line."""

    def format(self, record: logging.LogRecord) -> str:
        if getattr(record, "plain_block", False):
            return record.getMessage()
        return super().format(record)


class _HttpxRequestDebugFilter(logging.Filter):
    """Keep routine HTTP request summaries out of normal operational logs."""

    def filter(self, record: logging.LogRecord) -> bool:
        if record.name == "httpx" and record.levelno == logging.INFO:
            record.levelno = logging.DEBUG
            record.levelname = logging.getLevelName(logging.DEBUG)
        return True


def configure_logging() -> None:
    """Configure operational logs once, using a Docker-friendly stdout stream."""
    level_name = os.getenv("LOG_LEVEL", DEFAULT_LOG_LEVEL).upper()
    level = getattr(logging, level_name, logging.INFO)
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)
    handler.setFormatter(
        _PipelineFormatter(
            "%(asctime)s %(levelname)s %(name)s — %(message)s",
            datefmt="%Y-%m-%dT%H:%M:%S%z",
        )
    )
    logging.basicConfig(
        level=logging.DEBUG,
        handlers=[handler],
    )
    logging.getLogger("httpx").addFilter(_HttpxRequestDebugFilter())
