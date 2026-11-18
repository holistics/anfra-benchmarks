#!/bin/sh
# Set up Metabase for the benchmark: an admin, then the bench Postgres as a database. Safe to rerun.
# Bench-only credentials.
set -eu
MB=${MB:-http://127.0.0.1:58089}
EMAIL=admin@bench.invalid
PASSWORD=bench-Metabase-1

json() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

token=$(curl -sf "$MB/api/session/properties" | json 'd.get("setup-token") or ""')
if [ -n "$token" ] && [ "$(curl -sf "$MB/api/session/properties" | json 'd.get("has-user-setup")')" != "True" ]; then
  curl -sf -X POST "$MB/api/setup" -H 'Content-Type: application/json' -d "{
    \"token\": \"$token\",
    \"user\": {\"email\": \"$EMAIL\", \"password\": \"$PASSWORD\", \"first_name\": \"bench\", \"last_name\": \"bench\"},
    \"prefs\": {\"site_name\": \"bench\", \"allow_tracking\": false}
  }" >/dev/null
  echo "Metabase: admin created"
fi

session=$(curl -sf -X POST "$MB/api/session" -H 'Content-Type: application/json' \
  -d "{\"username\": \"$EMAIL\", \"password\": \"$PASSWORD\"}" | json 'd["id"]')

# add_database NAME JSON: adds the database unless one of that name exists.
add_database() {
  existing=$(curl -sf "$MB/api/database" -H "X-Metabase-Session: $session" \
    | json "[x['id'] for x in d['data'] if x['name'] == '$1']")
  if [ "$existing" = "[]" ]; then
    curl -sf -X POST "$MB/api/database" -H "X-Metabase-Session: $session" -H 'Content-Type: application/json' -d "$2" \
      | json '"Metabase: database " + d["name"] + " added, id " + str(d["id"])'
  else
    echo "Metabase: database $1 exists, id $existing"
  fi
}

# is_full_sync false: sync the schema only, with no scans of the tables' values, which would
# otherwise run in the background while the benchmark measures.
add_database bench_pg '{
  "engine": "postgres", "name": "bench_pg", "is_full_sync": false, "is_on_demand": false,
  "details": {"host": "127.0.0.1", "port": 55432, "dbname": "bench", "user": "bench", "password": "bench", "ssl": false}
}'
# ClickHouse's Metabase driver speaks its HTTP protocol.
add_database bench_ch '{
  "engine": "clickhouse", "name": "bench_ch", "is_full_sync": false, "is_on_demand": false,
  "details": {"host": "127.0.0.1", "port": 58123, "dbname": "bench", "user": "bench", "password": "bench", "ssl": false}
}'
