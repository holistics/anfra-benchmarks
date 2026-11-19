#!/bin/sh
# Load TPC-H at scale factor 1 into the benchmark Postgres (schema tpch), for the semantic group.
# DuckDB's TPC-H extension generates the data (into .bin/tpch-data, git-ignored), Postgres loads it
# with COPY. Run from the query folder, with Postgres up (docker compose up -d postgres). It
# replaces the schema each time. The tables the semantic group queries: region, nation, customer,
# orders, lineitem.
set -eu
DUCKDB=.bin/duckdb-venv/bin/python
DATA=.bin/tpch-data
PSQL="psql -X -q -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U bench -d bench"
export PGPASSWORD=bench

mkdir -p "$DATA"
"$DUCKDB" - "$DATA" <<'PY'
import sys, duckdb
out = sys.argv[1]
con = duckdb.connect()
con.execute("INSTALL tpch; LOAD tpch; CALL dbgen(sf = 1)")
for table in ["region", "nation", "customer", "orders", "lineitem"]:
    con.execute(f"COPY {table} TO '{out}/{table}.csv' (HEADER, DELIMITER ',')")
    print(table, con.execute(f"SELECT count(*) FROM {table}").fetchone()[0])
PY

$PSQL -f postgres/tpch.sql
for table in region nation customer orders lineitem; do
  $PSQL -c "\\copy tpch.$table FROM '$DATA/$table.csv' WITH (FORMAT csv, HEADER true)"
done
$PSQL -c "ANALYZE tpch.region, tpch.nation, tpch.customer, tpch.orders, tpch.lineitem"
echo "TPC-H loaded into schema tpch"
