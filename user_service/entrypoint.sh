#!/bin/sh
set -e

python migrate.py || echo "[entrypoint] migrate failed or skipped"
exec python app.py
