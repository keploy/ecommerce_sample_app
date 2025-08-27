#!/bin/sh
set -e

# Run DB migrations (best-effort on startup)
python migrate.py || echo "[entrypoint] migrate failed or skipped"

# Exec the app so it becomes PID 1 and receives signals
exec python app.py
