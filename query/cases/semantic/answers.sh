#!/bin/sh
# Write each semantic case's answer, <case>.csv, from its hand-written SQL, <case>.sql: the result
# every tool must return, value by value. The answers are generated (and git-ignored), since the
# listing cases' are large; the SQL is their definition. Run from the query folder, with the
# benchmark Postgres up and TPC-H loaded (postgres/tpch.sh).
set -eu
cd cases/semantic
for sql in *.sql; do
  PGPASSWORD=bench psql -X -q -v ON_ERROR_STOP=1 --csv -h 127.0.0.1 -p 55432 -U bench -d bench -f "$sql" -o "${sql%.sql}.csv"
  echo "${sql%.sql}: $(($(wc -l < "${sql%.sql}.csv") - 1)) rows"
done
