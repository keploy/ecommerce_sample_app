#!/bin/bash
set -eu

# Setup Keploy CA once (non-fatal if repeated)
if [ -f ./setup_ca.sh ]; then
  source ./setup_ca.sh || true
fi

exec ./order-service

