#!/usr/bin/env bash

set -e

# Go get Prometheus
wget -q https://github.com/prometheus/prometheus/releases/download/v2.16.0/prometheus-2.16.0.linux-amd64.tar.gz
tar xzf prometheus-2.16.0.linux-amd64.tar.gz
chmod +x ./prometheus-2.16.0.linux-amd64/prometheus

# Go get YQ
wget -q https://github.com/mikefarah/yq/releases/download/3.2.1/yq_linux_amd64
YQ="./yq_linux_amd64"
chmod +x $YQ

# Interpolate the variables in to the config file
cat /home/vcap/app/prometheus-config.yml \
| $YQ write - scrape_configs[0].basic_auth.username "${AIVEN_BASIC_AUTH_USER}" \
| $YQ write - scrape_configs[0].basic_auth.password "${AIVEN_BASIC_AUTH_PASSWORD}" \
> /home/vcap/app/prometheus-config-full.yml

cat /home/vcap/app/prometheus-config-full.yml

# Touch the service discovery file to make sure it exists
touch "${SERVICE_DISCOVERY_TARGET_PATH}"
cat <<EOF > "${SERVICE_DISCOVERY_TARGET_PATH}"
[
  {
    "labels": {},
    "targets": []
  }
]
EOF

cat "${SERVICE_DISCOVERY_TARGET_PATH}"

# Run Promtheus with the config file
./prometheus-2.16.0.linux-amd64/prometheus \
  --config.file=prometheus-config-full.yml \
  --web.listen-address="0.0.0.0:${PORT}"
