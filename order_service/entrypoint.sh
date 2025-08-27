#!/bin/bash
set -euo pipefail

# Setup CA once (non-fatal if repeated)
if [ -f ./setup_ca.sh ]; then
  source ./setup_ca.sh || true
fi

# Run DB migrations
python migrate.py || echo "[entrypoint] migrate failed or skipped"

# Exec app
exec python app.py
