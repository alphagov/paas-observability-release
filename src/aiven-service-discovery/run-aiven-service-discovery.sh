#!/usr/bin/env bash

set -e

bin/aiven-service-discovery \
  --aiven-api-token "${AIVEN_API_TOKEN}" \
  --aiven-project "${AIVEN_PROJECT}" \
  --aiven-prometheus-endpoint-id "${AIVEN_PROMETHEUS_ENDPOINT_ID}" \
  --service-discovery-target-path "${SERVICE_DISCOVERY_TARGET_PATH}" \
  --service-names-file "${SERVICE_NAMES_FILE}" \
  --prometheus-listen-port "${PROMETHEUS_LISTEN_PORT}" \
  --log-level 1
