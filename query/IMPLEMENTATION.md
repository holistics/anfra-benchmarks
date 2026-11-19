# Query benchmark: implementation

How the [plan](README.md) runs: the files, how each tool is driven and timed,
and how to run a batch.

## Files

| Path | What it is |
|---|---|
| `docker-compose.yml` | the warehouse and the tools that run as servers |
| `postgres/init.sql` | generates `bench_wide`: 1M rows, 50 typed columns, values derived from `id`, so every load is the same |
| `snowflake/` | `init.sql` creates the schema `ANFRA_BENCH` and `bench_wide` in Snowflake from the same formulas as Postgres, with checks against Postgres's values; `init.sh` runs it through anfra's `bench_sf` data source; `setup.py` connects every other tool from that data source |
| `postgres/tpch.sql`, `postgres/tpch.sh` | the semantic group's data: TPC-H at scale factor 1 in the schema `tpch` (region, nation, customer, orders, lineitem), with the spec's primary keys and no other index. `tpch.sh` generates it with DuckDB and loads it with `COPY` |
| `cube/model/` | Cube's model: TPC-H in YAML, a cube per table with its joins and primary key, a one-second refresh key on each (so no run is answered from Cube's cache), and the multi-stage measures for the share of total |
| `metricflow/` | MetricFlow's model: a minimal dbt project over TPC-H (a plain view per table in schema `mf`, a daily time spine MetricFlow requires, and the semantic models and metrics in `models/semantic.yml`), with bench-only credentials in `profiles.yml` |
| `cases/semantic/` | the semantic group's cases: per case, a hand-written SQL query (`<case>.sql`), its definition, and its result (`<case>.csv`, written by `answers.sh` and git-ignored, since the listings' are large), which every tool must return, value by value (`check.py` compares a result with it); and each tool's form of the case: `anfra/<case>.aql`, `metricflow.json` (the `mf query` arguments, or why a case is not run), `cube.json` (the REST query, and its result's members in the case's column order) |
| `clickhouse/init.sql` | creates `bench_wide` in ClickHouse with matching types, and copies its rows from Postgres on first start, so both hold the same values |
| `superset/` | Superset's image (6.1.0 with a Postgres driver), start script and config |
| `metabase/` | `setup.sh`: Metabase's first-run setup (an admin, then the bench Postgres), safe to rerun |
| `cases/` | for the report: the cases (`cases.jsonl`, from `.bin/runner cases`) and the first 20 rows of `bench_wide` (`bench_wide_head.csv`, from psql); regenerate both when the cases or `init.sql` change |
| `report/` | an anfra repo that reads `results/` and `cases/` with DuckDB, and charts them in a Data App (`report/apps/query-benchmark.html`); run `anfra serve` in it |
| `anfra-repo/` | the anfra repo the runner serves and queries: a data source per warehouse (`bench_pg`, `bench_ch`, `bench_sf`), no query cache. Its `data_sources.yml` holds credentials and is git-ignored: copy `data_sources.yml.example` and fill in Snowflake. Every tool's Snowflake connection is read from it |
| `runner/` | the Go runner (its own module, stdlib only): the cases, a driver per tool, the report |
| `results/<machine>/` | each machine's batches: runs (`batch<N>.jsonl`), write-up (`batch<N>.md`), and the run's log and `anfra serve` log (not committed) |

## How each tool is run and timed

| Tool | Path | What the runner does | Timed from → to |
|---|---|---|---|
| `psql` (Postgres's native CLI) | file | `psql -X -q --csv -o <file> -c <sql>` | start → exit |
| `snow` (Snowflake's CLI) | file | `snow sql -c bench -q <sql> -o output_format=csv -o quiet=true -o output_file=<file>`: the CLI writes the file itself | start → exit |
| `clickhouse-client` (ClickHouse's native CLI) | file | `clickhouse client --query "<sql> INTO OUTFILE '<file>' TRUNCATE FORMAT CSVWithNames"`: the client writes the file itself | start → exit |
| `usql` | file | `usql -X -q --csv -o <file> -c <sql> <dsn>` | start → exit |
| `anfra-cli-cold` | file | `anfra query export --lang sql -o <file>`, in a copy of `anfra-repo/` that no server runs for: the command starts its own sidecars | start → exit |
| `anfra-cli-warm` | file | the same command in `anfra-repo/`, where the runner runs `anfra serve`: the command is forwarded to it, and downloads the file | start → exit |
| `metricflow` | file | `mf query --quiet <the case's arguments> --csv <file>`, in `metricflow/` (semantic group) | start → exit |
| `cube` | http | `POST /cubejs-api/v1/load` with the case's query and a JWT signed with Cube's API secret; while Cube answers "Continue wait", it asks again, as Cube's client does (semantic group) | first request → answer parsed |
| `anfra-http` | http | `POST /api/core.query` with `Accept: application/x-ndjson`, as the SDK sends it; reads and parses every event | request → last event parsed |
| `superset` | http | `POST /api/v1/sqllab/execute/`, synchronous (`runAsync: false`), with a JWT from one login; parses the whole JSON answer | request → answer parsed |
| `metabase` | http | `POST /api/dataset` with a native query, as its question editor runs SQL, with a session from one login; parses the whole JSON answer | request → answer parsed |

Each path compares like with like:

- **File:** every tool writes the whole result to a CSV file. anfra's command
  for that is `anfra query export`. `anfra query --json` returns a page of at
  most 10,000 rows, so a 1M-row case would take 100 calls.
- **HTTP:** both tools use the path their frontend uses to show a result:
  anfra's `core.query`, and Superset's SQL Lab execute. Neither is an export.
  Superset's streaming CSV export would be compared with anfra's export, on a
  path of its own.

In the semantic group, the anfra tools run the case's AQL (`cases/semantic/anfra/<case>.aql`) on the `tpch` dataset, with the same commands and endpoint.

After the timer stops, the runner checks the result's shape: the CSV file's
header and rows, the stream's `header` and `rows` events, or the answer's
`columns` and `data`. A run whose rows or columns differ from its case's is a
failure.

The HTTP client keeps connections alive between runs and asks for gzip, as a
browser does.

## Tool settings

- **Superset 6.1.0**: synchronous SQL Lab, no row cap (`SQL_MAX_ROW`,
  `DISPLAY_MAX_ROW` at 2M), no caches, one gunicorn worker with 20 threads (the
  image's default). It runs on the host network, so it reaches Postgres the way
  the CLI tools do (`127.0.0.1:55432`). It rewrites `LIMIT n` to `LIMIT n+1`, to
  detect truncation, and returns n rows.
- **Metabase v0.65.5.1**: its metadata in its embedded H2 database, query
  caching off, row limits raised to its maximum (1,048,575) so `narrow_1m`
  returns whole, a 3 GB Java heap. On the host network, like Superset. The
  bench database is added without value scans (`is_full_sync: false`), so no
  background scan of `bench_wide` runs while the benchmark measures.
- **anfra**: the one on `PATH`. `anfra-repo/` sets no query cache, and every
  request also sends `cache.duration: 0s`. `anfra serve` runs with `--no-apps
  --no-mcp`, and logs to `anfra-serve.log` beside the `-out` file.
- **usql** v0.21.6, built with its default drivers and the ClickHouse and
  Snowflake drivers (`-tags clickhouse,snowflake`).
- **snow** (the Snowflake CLI) 3.34.0, in its own Python environment in
  `.bin/snow-venv`.
- **clickhouse-client** 26.8.39.19, the server's version: the `clickhouse`
  binary from ClickHouse's GitHub release, checked against the release's
  `.sha512`. It writes NULL as `\N`, ClickHouse's CSV convention.
- **ClickHouse 26.8 (LTS)**: `bench_wide` as a MergeTree ordered by `id`, times
  in UTC. Its native protocol (59000) serves clickhouse-client, usql and anfra;
  its HTTP port (58123) serves Superset (`clickhouse-connect`) and Metabase (its
  built-in driver). A scan with LIMIT and no ORDER BY may return other rows than
  Postgres's first ones, in another order: the rows' shape, which is all the
  check compares, is the same.
- **anfra's AQL compiler is v2** in `anfra-repo/` (`aql.compiler.version`). It
  compiles the semantic group's AQL; the SQL group's queries are SQL, which it
  doesn't touch.
- **The listing cases** list rows without aggregating: anfra with an explore of
  dimensions only, Cube with `ungrouped: true`, MetricFlow with group-bys and no
  metric (its distinct values; the line's entity makes each row one line item).
- **Postgres 16**: `shared_buffers=1GB`, so the table stays in memory, and 1 GB
  of shared memory (`shm_size`): Docker's default 64 MB runs out on TPC-H's
  parallel joins ("could not resize shared memory segment").
- **Telemetry is off for every tool**: Superset, Metabase and Cube by their
  settings; dbt's usage tracking for MetricFlow (`send_anonymous_usage_stats:
  false`, and `DBT_SEND_ANONYMOUS_USAGE_STATS=false` when the runner calls `mf`),
  which on a slow network added about 10 s before each `mf query`. anfra
  exports nothing unless an OpenTelemetry endpoint is set, and none is.
- **Snowflake** (batch 4, parked until it can run close to the account, see the
  [plan](README.md#batches)): the account's `TEST_DB` database, schema `ANFRA_BENCH` (made by
  `snowflake/init.sql`), warehouse `DEMO_WH`, region AWS `ap-southeast-2`
  (Sydney). Every tool logs in with the same key pair, as the same user with its
  default role. On Snowflake the cases query `ANFRA_BENCH.bench_wide`, since
  anfra's Snowflake data source has no schema key. **The result cache stays on**
  (Snowflake's default): the benchmark times each tool, not the warehouse, and
  every run still fetches its result over the network. The cases are a subset:
  `select1`, `narrow_1k`, `narrow_100k`, `medium_100k`, `wide_1k`.

## Where each tool comes from

Every version is pinned. The server tools run from Docker Hub images; the CLI
tools and the runner are installed into `.bin/` (git-ignored) by the commands in
[Running a batch](#running-a-batch), except psql and anfra, which come from the
machine.

| Tool | Version | Distribution | Downloaded from | Installed by |
|---|---|---|---|---|
| Postgres | 16 | official Docker image `postgres:16` | Docker Hub | `docker compose` |
| ClickHouse server | 26.8.39.19 (LTS) | official Docker image `clickhouse/clickhouse-server` | Docker Hub | `docker compose` |
| Snowflake | (the service) | – | – | an account; `snowflake/init.sh` creates the schema and table, `snowflake/setup.py` connects the tools |
| DuckDB | 2.0.0 | the `duckdb` Python package, on the machine's Python 3.12, with its `tpch` extension (to generate TPC-H) | PyPI; the extension from DuckDB's extension server (extensions.duckdb.org), on `INSTALL tpch` | `uv`, into `.bin/duckdb-venv` |
| MetricFlow | 0.214.0, through `dbt-metricflow` 0.16.0 | the `dbt-metricflow` package with dbt-core 1.12.7 and dbt-postgres 1.11.0, on the machine's Python 3.12 | PyPI | `uv`, into `.bin/mf-venv` |
| psql | 15.18 | `postgresql-client-15` package, `15.18-1.pgdg24.04+1` | the PostgreSQL apt repository, apt.postgresql.org | the machine's package manager |
| clickhouse-client | 26.8.39.19 | the official static build, `clickhouse-common-static-26.8.39.19-amd64.tgz`, checked against its `.sha512` | GitHub Releases, ClickHouse/ClickHouse, tag `v26.8.39.19-lts` | `curl` and `tar`, into `.bin/clickhouse` |
| snow (Snowflake CLI) | 3.34.0 | the `snowflake-cli` package, on the machine's Python 3.12 | PyPI | `uv`, into `.bin/snow-venv` |
| usql | v0.21.6 | built from source, with `-tags clickhouse,snowflake` | the Go module proxy (proxy.golang.org), `github.com/xo/usql` | `go install`, into `.bin/usql` (Go 1.27.3, with Go's own toolchain download of 1.26.11, which usql asks for) |
| anfra | 0.22.0 | the release binary for linux/amd64, which embeds its sidecars | GitHub Releases, via anfra's install script (anfra.ai/install.sh) | the install script, into `~/.anfra/bin` |
| Superset | 6.1.0 | official Docker image `apache/superset:6.1.0`, with drivers added: `psycopg2-binary` 2.9.10, `clickhouse-connect` 1.13.0, `snowflake-sqlalchemy` 1.11.1 (for SQLAlchemy 1.4, which Superset uses; it brings `snowflake-connector-python` 4.8.0) | Docker Hub; the drivers from PyPI | `docker compose build` (`superset/Dockerfile`) |
| Cube | v1.8.28 | official Docker image `cubejs/cube:v1.8.28`, production mode, cache and queue in memory (no Cube Store, since no pre-aggregations) | Docker Hub | `docker compose` |
| Metabase | v0.65.5.1 | official Docker image `metabase/metabase:v0.65.5.1`, with its built-in Postgres, ClickHouse and Snowflake drivers | Docker Hub | `docker compose` |
| the runner | – | this repo's `runner/`, standard library only | – | `go build`, into `.bin/runner` |

## Running a batch

From this folder. Needs Docker, Go, uv, psql and anfra on `PATH`. Everything else
it installs into `.bin/` (git-ignored), with the commands below. Every tool reaches
every warehouse; `-db` picks the one a run measures.

```sh
docker compose up -d --build        # Postgres (55432), ClickHouse (59000, 58123), Superset (58088), Metabase (58089), Cube (58100), on 127.0.0.1
metabase/setup.sh                   # once Metabase answers on /api/health
GOBIN=$PWD/.bin go install -tags clickhouse,snowflake github.com/xo/usql@v0.21.6
go -C runner build -o ../.bin/runner .

# clickhouse-client: the release's binary, at the server's version, checked against its checksum
V=26.8.39.19; T=clickhouse-common-static-$V-amd64.tgz
curl -sSfLO https://github.com/ClickHouse/ClickHouse/releases/download/v$V-lts/$T
curl -sSfL https://github.com/ClickHouse/ClickHouse/releases/download/v$V-lts/$T.sha512 | sed "s/ .*/  $T/" | sha512sum -c
tar -xzf $T && cp clickhouse-common-static-$V/usr/bin/clickhouse .bin/ && rm -rf $T clickhouse-common-static-$V

# DuckDB, to generate TPC-H for the semantic group; then load it into Postgres
uv venv .bin/duckdb-venv --python 3.12 && uv pip install --python .bin/duckdb-venv/bin/python duckdb==2.0.0
postgres/tpch.sh
cases/semantic/answers.sh            # each semantic case's answer, from its hand-written SQL

# MetricFlow, with dbt; then its views, time spine and semantic manifest
uv venv .bin/mf-venv --python 3.12 && uv pip install --python .bin/mf-venv/bin/python "dbt-metricflow[dbt-postgres]==0.16.0" dbt-core==1.12.7 dbt-postgres==1.11.0
(cd metricflow && DBT_PROFILES_DIR=. ../.bin/mf-venv/bin/dbt run && DBT_PROFILES_DIR=. ../.bin/mf-venv/bin/dbt parse)

# snow (the Snowflake CLI), in its own environment, with uv
uv venv .bin/snow-venv --python 3.12 && uv pip install --python .bin/snow-venv/bin/python snowflake-cli==3.34.0

# Snowflake: fill in bench_sf (copy anfra-repo/.anfra/data_sources.yml.example), then
snowflake/init.sh                              # the schema and bench_wide, through anfra; check its results against the comments
.bin/snow-venv/bin/python snowflake/setup.py   # snow, usql, Superset and Metabase, from bench_sf

mkdir -p results/<machine>                         # one folder per machine, such as linux-ryzen7-7840u
.bin/runner -db postgres -batch <N> -out results/<machine>/batch<N>.jsonl     # every tool and case: 1 warm-up round, 5 measured
.bin/runner -group semantic -batch <N> -out results/<machine>/batch<N>.jsonl  # the semantic group, on TPC-H in Postgres
.bin/runner report results/<machine>/batch<N>.jsonl            # the table: median (slowest) ms per case and tool
```

`-tools`, `-cases`, `-rounds`, `-warmup` and `-skip tool:case,…` narrow a run.
In `-tools`, `native` is the warehouse's own CLI: psql, clickhouse-client or snow.
`-group` is `sql` (the default) or `semantic`; each has its own tools by default.
A semantic run's values are checked against its case's answer too, and a case
a tool doesn't run is recorded once, as `not_run` with the reason, instead of
runs.
`-db` is `postgres`, `clickhouse` or `snowflake`.
Runs are appended to the `-out` file as JSON Lines, one per measured run:

```json
{"batch":1,"db":"postgres","tool":"superset","path":"http","case":"wide_100k",
 "round":3,"ms":412.3,"rows":100000,"cols":30,"ok":true}
```

`-batch` numbers the batch in every run it writes, as the report and the plan's
batches use. Ctrl-C stops between runs, and keeps the runs already written. A
rerun starts from round 1 again: resuming is planned.
