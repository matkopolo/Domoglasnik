"""Command-line entry point for the real-estate ETL application."""

import sys

from .application import main

if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1) from None
