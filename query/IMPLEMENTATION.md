# Query benchmark: implementation

How the [plan](README.md) runs: the files, how each tool is driven and timed,
and how to run a batch.

## Files

| Path | What it is |
|---|---|
| `docker-compose.yml` | the warehouse and the tools that run as servers |
| `postgres/init.sql` | generates `bench_wide`: 1M rows, 50 typed columns, values derived from `id`, so every load is the same |
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
| `clickhouse-client` (ClickHouse's native CLI) | file | `clickhouse client --query "<sql> INTO OUTFILE '<file>' TRUNCATE FORMAT CSVWithNames"`: the client writes the file itself | start → exit |
| `usql` | file | `usql -X -q --csv -o <file> -c <sql> <dsn>` | start → exit |
| `anfra-cli-cold` | file | `anfra query export --lang sql -o <file>`, in a copy of `anfra-repo/` that no server runs for: the command starts its own sidecars | start → exit |
| `anfra-cli-warm` | file | the same command in `anfra-repo/`, where the runner runs `anfra serve`: the command is forwarded to it, and downloads the file | start → exit |
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
- **usql** v0.21.6, built with its default drivers and the ClickHouse driver
  (`-tags clickhouse`).
- **clickhouse-client** 26.8.39.19, the server's version: the `clickhouse`
  binary from ClickHouse's GitHub release, checked against the release's
  `.sha512`. It writes NULL as `\N`, ClickHouse's CSV convention.
- **ClickHouse 26.8 (LTS)**: `bench_wide` as a MergeTree ordered by `id`, times
  in UTC. Its native protocol (59000) serves clickhouse-client, usql and anfra;
  its HTTP port (58123) serves Superset (`clickhouse-connect`) and Metabase (its
  built-in driver). A scan with LIMIT and no ORDER BY may return other rows than
  Postgres's first ones, in another order: the rows' shape, which is all the
  check compares, is the same.
- **Postgres 16**: `shared_buffers=1GB`, so the table stays in memory.

## Running a batch

From this folder. Needs Docker, Go, psql and anfra on `PATH`. Every tool reaches
every warehouse; `-db` picks the one a run measures.

```sh
docker compose up -d --build        # Postgres (55432), ClickHouse (59000, 58123), Superset (58088), Metabase (58089), on 127.0.0.1
metabase/setup.sh                   # once Metabase answers on /api/health
GOBIN=$PWD/.bin go install -tags clickhouse github.com/xo/usql@v0.21.6
go -C runner build -o ../.bin/runner .

# clickhouse-client: the release's binary, at the server's version, checked against its checksum
V=26.8.39.19; T=clickhouse-common-static-$V-amd64.tgz
curl -sSfLO https://github.com/ClickHouse/ClickHouse/releases/download/v$V-lts/$T
curl -sSfL https://github.com/ClickHouse/ClickHouse/releases/download/v$V-lts/$T.sha512 | sed "s/ .*/  $T/" | sha512sum -c
tar -xzf $T && cp clickhouse-common-static-$V/usr/bin/clickhouse .bin/ && rm -rf $T clickhouse-common-static-$V

mkdir -p results/<machine>                         # one folder per machine, such as linux-ryzen7-7840u
.bin/runner -db postgres -batch <N> -out results/<machine>/batch<N>.jsonl     # every tool and case: 1 warm-up round, 5 measured
.bin/runner report results/<machine>/batch<N>.jsonl            # the table: median (slowest) ms per case and tool
```

`-tools`, `-cases`, `-rounds`, `-warmup` and `-skip tool:case,…` narrow a run.
In `-tools`, `native` is the warehouse's own CLI: psql, or clickhouse-client.
Runs are appended to the `-out` file as JSON Lines, one per measured run:

```json
{"batch":1,"db":"postgres","tool":"superset","path":"http","case":"wide_100k",
 "round":3,"ms":412.3,"rows":100000,"cols":30,"ok":true}
```

`-batch` numbers the batch in every run it writes, as the report and the plan's
batches use. Ctrl-C stops between runs, and keeps the runs already written. A
rerun starts from round 1 again: resuming is planned.
