# Query benchmark

How long anfra takes to run a query and hand over its whole result, against
popular open-source tools, on the same warehouse with the same query.

This README is the plan: what is measured, in what order, and why, and where
the results are. [IMPLEMENTATION.md](IMPLEMENTATION.md) says how it runs, and
how to run a batch yourself.

> **How fair is it?** For each tool, we use the best way we know to run a query
> and get its whole result, on the path its own users take: its CLI writing a
> CSV file, or the API its own frontend calls. Caches are off, and row limits
> raised so every case returns whole. We may be missing a better method, or a
> setting that would make a tool faster. Each tool's exact setup is in
> [IMPLEMENTATION.md](IMPLEMENTATION.md). If you know a better way, tell us, and
> we will measure it.

## Results so far

| Batch | Warehouse | Tools | Write-up |
|---|---|---|---|
| 1 | Postgres | psql, usql, anfra (CLI cold, CLI warm, HTTP), Superset | [batch1.md](results/linux-ryzen7-7840u/batch1.md) |
| 2 | Postgres | psql, usql, anfra (CLI cold, CLI warm, HTTP), Superset, **Metabase** | [batch2.md](results/linux-ryzen7-7840u/batch2.md) |
| 3 | ClickHouse | clickhouse-client, usql, anfra (CLI cold, CLI warm, HTTP), Superset, Metabase | [batch3.md](results/linux-ryzen7-7840u/batch3.md) |

All ran on one machine, `linux-ryzen7-7840u` (AMD Ryzen 7 PRO 7840U, 27 GiB,
Linux).

## Viewing the results

Three ways, from quickest to most detailed. None needs Docker or a warehouse.

1. **Read a batch's write-up**, `results/<machine>/batch<N>.md`: the table of
   medians, the versions, the machine, and what stands out.
2. **Open the report**, an anfra Data App over every batch's runs. It needs
   only the `anfra` CLI installed:

   ```sh
   cd query/report
   anfra serve          # then open the address it prints, at /apps/query-benchmark
   ```

   Pick a machine, a warehouse (Postgres by default) and one of its batches
   (its newest by default), and switch
   the file and HTTP paths on or off. The page shows each tool's median per
   case, a comparison of the tools you pick against a baseline (Superset and
   Metabase against anfra HTTP by default), every tool and case in a table, every
   measured run with filters, and a preview of what each case returns.
3. **Query the raw runs** with anfra, from the report's repo. Each batch's
   runs are in `results/<machine>/batch<N>.jsonl`, one JSON line per measured
   run. There are two ways in:

   - **AQL**, on the `benchmark` dataset, which models every batch's runs with
     their machine, batch, warehouse, path, case and tool, and measures such as
     the median:

     ```sh
     cd query/report
     anfra show benchmark      # its fields and measures
     anfra query --pretty --dataset benchmark 'explore {
       dimensions { case_order: runs.case_order, case_name: runs.case_name, tool: runs.tool }
       measures { median_ms: runs.median_ms, slowest_ms: runs.slowest_ms }
       filters { runs.machine == "linux-ryzen7-7840u", runs.batch == 2, runs.db == "postgres" }
       sorts { case_order asc, tool asc }
     }'
     ```

   - **SQL**, on the `results` data source, an in-memory DuckDB that reads the
     files as they are:

     ```sh
     cd query/report
     anfra query --pretty --lang sql -s results "SELECT \"case\", tool, median(ms) AS median_ms, max(ms) AS slowest_ms
       FROM read_json_auto('../results/linux-ryzen7-7840u/batch2.jsonl')
       WHERE ok GROUP BY ALL ORDER BY \"case\", tool"
     ```

Compare tools only within one machine, one batch and one path.

## Running the benchmark

To measure a batch yourself, on your own machine:

1. **Start the warehouse and the server tools** (Postgres, Superset, Metabase)
   with Docker Compose, and set Metabase up.
2. **Build the runner and usql**, with Go.
3. **Run the batch** into `results/<your machine>/batch<N>.jsonl`. A full
   Postgres batch takes about 7 minutes.
4. **Write it up** in `results/<your machine>/batch<N>.md`, and link it from
   [Results so far](#results-so-far). The report shows it without changes.

The commands, and what each tool needs, are in
[IMPLEMENTATION.md](IMPLEMENTATION.md#running-a-batch). Name your machine's
folder for its OS and CPU (`macos-m3-pro`), so runs from different machines
stay apart.

## This folder

```
query/
├── README.md            this plan, and where the results are
├── IMPLEMENTATION.md    how each tool is run and timed; how to run a batch
├── results/             the results, one folder per machine
│   └── <machine>/       batch<N>.jsonl (every measured run), batch<N>.md (the write-up)
├── report/              the anfra repo for the report: its Data App, models and datasets
├── cases/               the cases, and the first rows of the table they scan, for the report
├── runner/              the Go program that runs the tools, times them and checks their results
├── docker-compose.yml   the warehouse and the tools that run as servers
├── postgres/            init.sql: generates the table every case scans
├── clickhouse/          init.sql: the same table in ClickHouse, copied from Postgres
├── superset/            Superset's image, start script and config
├── metabase/            setup.sh: Metabase's first-run setup
└── anfra-repo/          the anfra repo under test: one data source, no models
```

To read the results you need only `results/`, and `report/` with `cases/` for
the Data App. The rest is for running the benchmark.

## Questions it answers

1. **SQL group:** for the same SQL, how much time does each tool add on top of
   the warehouse, by result size, when the result goes to a file (CLI) or to a
   frontend (HTTP)?
2. **Semantic group:** for the same semantic query, how long does each
   semantic layer take to compile it to SQL, and to deliver its result?

## The matrix

### Warehouses

| Warehouse | Where | Why |
|---|---|---|
| **Postgres** 16 | local Docker | the common row store; every tool supports it |
| **ClickHouse** | local Docker | a fast columnar engine, where the tool's own cost shows most |
| **Snowflake** | cloud, one region, smallest warehouse | what customers run; adds network and costs credits |

### Tools and their paths

A tool delivers a result over a **path**, and is compared only with tools on
the same path:

- **CLI → CSV file:** the tool's CLI writes the whole result to a CSV file
  with a header row. Timed from process start to exit.
- **HTTP:** the tool answers the request its own frontend sends. Timed until
  the whole answer is read and parsed. Formats differ: anfra streams NDJSON,
  the BI tools return one JSON document.

Most tools have only one of the two. anfra and Cube have both.

| Tool | Group | CLI → CSV file | HTTP |
|---|---|---|---|
| the warehouse's native CLI: psql, clickhouse-client, snow | floor | ✅ `--csv` or its equivalent | ✗ none |
| usql | SQL | ✅ `--csv -o file` | ✗ none |
| **anfra** | SQL, semantic | ✅ `anfra query export`, cold (no server) and warm (forwarded to `anfra serve`) | ✅ `core.query`, streamed NDJSON |
| Superset 6.1 | SQL | ✗ no CLI | ✅ SQL Lab execute, synchronous |
| Metabase v0.65 | SQL | ✗ no CLI | ✅ `/api/dataset`, native query |
| Cube | semantic | ✅ through psql, on its SQL API ¹ | ✅ REST `/load` |
| MetricFlow | semantic | ✅ `mf query --csv` | ✗ none (dbt Cloud only) |
| Rill | semantic | ✗ not planned | ✅ runtime API |

For the semantic group, anfra runs AQL on both paths.

### Which warehouse each tool runs on

| Tool | Postgres | ClickHouse | Snowflake |
|---|---|---|---|
| native CLI | ✅ psql | ✅ clickhouse-client | ✅ snow (or snowsql) |
| usql | ✅ | ✅ built with `-tags clickhouse` | ✅ ² |
| **anfra** | ✅ | ✅ | ✅ |
| Superset | ✅ | ✅ | ✅ |
| Metabase | ✅ | ✅ built-in driver | ✅ |
| Cube | ✅ | ✅ | ✅ |
| MetricFlow | ✅ | ✗ no adapter | ✅ |
| Rill | ✗ not a query engine | ✅ live ³ | ✅ live ³ |

1. Cube's SQL API speaks the Postgres protocol, so psql can write its result to
   a CSV file. Optional: without it, Cube is HTTP only.
2. usql needs its Snowflake driver built in, with `-tags snowflake`.
3. Rill by default copies data into its own engine (DuckDB) and queries the
   copy. Run live, it queries the warehouse itself, as every other tool here
   does. Only the live mode is comparable. To confirm: that the Rill release
   used runs live on Snowflake (older releases only copied from it).

Noted for later, not planned: Superset's streaming CSV export
(`POST /api/v1/sqllab/export_streaming/`), a file over HTTP, the closest match
to `anfra query export` behind a server.

## Cases

### SQL group: result shape

A SQL-only tool passes the query through, so what grows its cost is the
result's shape, not the query's complexity. Every case is a plain scan of one
generated table, `bench_wide` (1M rows, 50 typed columns, about 10% NULLs),
loaded the same into every warehouse.

| Case | Columns | Rows |
|---|---|---|
| `select1` | 1 (`SELECT 1`: the tool's fixed cost per query) | 1 |
| `narrow_1`, `narrow_1k`, `narrow_100k`, `narrow_1m` | 3: bigint, short text, numeric | 1, 1k, 100k, 1M |
| `medium_100k` | 10: one of each type | 100k (an average query) |
| `wide_1`, `wide_1k`, `wide_100k` | 30: three of each type | 1, 1k, 100k |

The types are integer, bigint, numeric, double, short text, long text, date,
timestamp, timestamptz and boolean. Some are much costlier to decode and
encode than others (numeric, timestamps, long text), which is why every width
mixes them.

`narrow_1m` is anfra's default maximum over HTTP (1M rows; also at most 256
MiB).

On **Snowflake**, a subset, to bound credits and time: `select1`,
`narrow_1k`, `narrow_100k`, `medium_100k`, `wide_1k`.

### Semantic group: query shape

Here the query's shape matters: it is what the semantic layer compiles. The
data is TPC-H at scale factor 1, modelled the same way in each tool. To be
designed in its own batch; a first draft:

| Case | What it exercises | Rows out |
|---|---|---|
| `total` | one measure, no dimension: the compile floor | 1 |
| `by_month` | one measure by a date dimension, truncated | ~80 |
| `by_nation` | measures across a join (orders → customer → nation) | 25 |
| `filtered` | several measures, filtered on a dimension of a joined model | ~100 |
| `share_of_total` | a measure as a share of its total (window or level of detail) | 25 |
| `by_customer` | one measure by a high-cardinality dimension | ~100k |

A tool that can't express a case is recorded as "unsupported", not left out.

Each case is measured twice: **compile only** (anfra `query compile`, Cube's
`/sql`, `mf query --explain`) and **end to end**.

## Method

These hold for every batch.

- **Timing:** the file path, from process start to exit with the CSV file
  fully written; the HTTP path, from request sent to the whole answer read and
  parsed, as a frontend would.
- **The floor:** a tool's overhead is its time minus the native CLI's, for the
  same case. No warehouse time is subtracted: every tool sends the same SQL.
- **Correctness:** each result's rows and columns must match its case, or the
  run is a failure, and left out of the timings.
- **No caches:** every tool's result cache is off.
- **Interleaving:** each round runs every (tool, case) once, in a shuffled
  order, so drift during a run spreads across all tools. Warm-up rounds first,
  not recorded.
- **Rounds:** 1 warm-up, then 5 measured runs of every case. Reported as the
  median, with the slowest of the 5 beside it.
- **Versions:** every tool's version is pinned, and recorded with its results.
- **Machine:** the same machine for a batch; other containers stopped where
  possible. Its CPU, memory and OS are recorded with the results.

## Batches

Results come in batches, each one usable on its own. A warehouse's published
numbers come from one run of all its tools together, interleaved, once they
are all ready: tools from separate runs are compared only against their own
run's floor.

| Batch | Status | Warehouse | Tools | Cells (tools × cases) | Time |
|---|---|---|---|---|---|
| **1** | ✅ done | Postgres | psql, usql, anfra (cold, warm, HTTP), Superset | 6 × 9 = 54 | 6 min |
| **2** | ✅ done | Postgres | + Metabase: the full Postgres SQL run | 7 × 9 = 63 | 7 min |
| **3** | ✅ done | ClickHouse | clickhouse-client, usql, anfra ×3, Superset, Metabase | 7 × 9 = 63 | ~10 min |
| **4** | next | Snowflake | snow, usql, anfra ×3, Superset, Metabase | 7 × 5 = 35 | ~15 min, a few credits |
| **5** | | Postgres | semantic: anfra, Cube, MetricFlow | 3 tools × 6 cases × 2 (compile, end to end) = 36 | ~10 min |
| **6** | | ClickHouse | semantic: anfra, Cube, Rill | 36 | ~10 min |
| **7** | | Snowflake | semantic: anfra, Cube, MetricFlow, Rill | 4 × 6 × 2 = 48 | ~15 min, a few credits |

In all: 335 cells. The time is mostly the large cases on the slower tools
(in trial runs, usql and Superset spent about 70% of it), not the number of
cells.

Before batch 5, the semantic cases and models need designing: a batch of
their own, with no measurement.

## Runner work, by batch

- ✅ **Batch 2:** a Metabase adapter, and Metabase in the compose file.
- ✅ **Batch 3:** the warehouse as a runner setting (`-db`), ClickHouse with
  `bench_wide` copied from Postgres, and each tool's ClickHouse connection.
- **Resume**, not done yet. On start, the runner would read its
  results file and run only the cells short of their rounds, so a run can be
  stopped (Ctrl-C) and continued. Warm-ups run again on resume, since
  processes restart cold.
- **Batch 4:** Snowflake credentials from the environment, never in the repo;
  the subset of cases.
- **Batch 5:** semantic cases (a query per tool per case), compile-only
  timing, and adapters for Cube, MetricFlow and Rill.

## Results

See [Results so far](#results-so-far) and [Viewing the results](#viewing-the-results).
Results are kept per machine, so runs on other machines (a Mac) sit beside
these. A final summary will compare anfra with each tool, per warehouse and
path.

## Open questions

- Which Snowflake account and region, and who pays for the credits?
- Are the results for internal use only, or to be published? Publishing
  needs a fairness review of each tool's settings by someone who knows the
  tool.
