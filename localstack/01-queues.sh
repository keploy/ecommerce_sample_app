#!/usr/bin/env bash
set -euo pipefail
awslocal sqs create-queue --queue-name order-events >/dev/null 2>&1 || true
