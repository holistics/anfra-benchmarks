#!/bin/sh
# Prepare Superset's metadata, an admin, and the bench warehouse, then serve.
set -e
superset db upgrade
superset fab create-admin --username admin --password admin \
  --firstname bench --lastname bench --email admin@bench.invalid || true
superset init
superset set-database-uri -d bench_pg -u postgresql://bench:bench@127.0.0.1:55432/bench
superset set-database-uri -d bench_ch -u clickhousedb://bench:bench@127.0.0.1:58123/bench
exec /usr/bin/run-server.sh
