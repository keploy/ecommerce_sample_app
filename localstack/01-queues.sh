#!/usr/bin/env sh
set -eu
awslocal sqs create-queue --queue-name order-events >/dev/null 2>&1 || true
