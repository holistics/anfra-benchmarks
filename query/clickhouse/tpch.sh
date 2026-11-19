#!/bin/sh
# Copy TPC-H from the benchmark Postgres (schema tpch, loaded by postgres/tpch.sh) into ClickHouse
# (database tpch), through ClickHouse's postgresql() table function. Run from the query folder, with
# both up. It replaces the tables each time.
set -eu
CH=".bin/clickhouse client --host 127.0.0.1 --port 59000 --user bench --password bench"
$CH --multiquery < clickhouse/tpch.sql
for table in region nation customer orders lineitem; do
  # From inside the ClickHouse container, Postgres is postgres:5432 on the compose network.
  $CH --query "INSERT INTO tpch.$table SELECT * FROM postgresql('postgres:5432', 'bench', '$table', 'bench', 'bench', 'tpch')"
  echo "$table: $($CH --query "SELECT count() FROM tpch.$table")"
done
