# Batch 5: Postgres, semantic group

Postgres (TPC-H at scale factor 1) × anfra (CLI cold, CLI warm, HTTP),
MetricFlow, Cube. Each tool runs each case as a semantic query in its own
language, end to end: 6 aggregate cases and 6 listing cases. Raw runs:
[batch5.jsonl](batch5.jsonl). The cases, and why each was picked, are in the
[plan](../../README.md#semantic-group-query-shape).

- **Run:** 2026-11-19. 1 warm-up round, then 5 measured rounds, tools and
  cases shuffled every round. 275 measured runs, every one returning its
  case's answer value by value (`cases/semantic/<case>.sql`), and 5 cases
  recorded as not run.
- **Versions:** anfra 0.22.0 with AQL compiler v2, MetricFlow 0.214.0
  (`dbt-metricflow` 0.16.0, dbt-core 1.12.7, dbt-postgres 1.11.0), Cube
  v1.8.28, Postgres 16.
- **Machine:** AMD Ryzen 7 PRO 7840U (16 threads), 27 GiB, Linux 7.0. Not
  isolated: other containers were running, idle.
- **How each tool is run:** see [IMPLEMENTATION.md](../../IMPLEMENTATION.md).
  anfra runs AQL on the `tpch` dataset; MetricFlow runs `mf query`, a new
  process each call, writing a CSV file; Cube runs its REST API.

## Results

Median ms, with the slowest of the 5 runs in brackets. Compare only within a
path. The last column is the hand-written SQL that defines each case's answer,
run once with psql, as a guide to how much of each time is Postgres's.

| Case | anfra CLI, cold | anfra CLI, warm | MetricFlow | anfra HTTP | Cube | Hand-written SQL (psql, one run) |
|---|---:|---:|---:|---:|---:|---:|
| | *file* | *file* | *file* | *http* | *http* | |
| **Aggregates** | | | | | | |
| `total` | 1057 (1135) | 416 (438) | 2209 (2564) | 400 (415) | 418 (458) | 426 |
| `by_month` | 1842 (2109) | 1251 (1369) | 3033 (3037) | 1411 (1494) | 1240 (1272) | 1334 |
| `by_nation` | 2112 (2455) | 1539 (1850) | not run ¹ | 1492 (1846) | 1520 (1675) | 1537 |
| `fan_out` | 1264 (1278) | 652 (660) | not run ¹ | 632 (656) | 635 (662) | 668 |
| `share_of_total` | 2452 (2562) | 1913 (2295) | not run ² | 1919 (2203) | 2998 (3157) | 1557 |
| `by_customer` | 3004 (3046) | 2394 (2583) | 4716 (4731) | 2412 (2443) | 2591 (2690) | 2428 |
| **Listings** | | | | | | |
| `list_1k` (2 columns) | 623 (628) | 21 (24) | 1788 (1815) | 19 (24) | 8 (456) | 30 |
| `list_10k` (2) | 626 (709) | 39 (51) | 1816 (2009) | 31 (35) | 28 (30) | 33 |
| `list_100k` (2) | 922 (1019) | 225 (278) | 2486 (2508) | 209 (270) | 279 (305) | 85 |
| `list_1m` (2) | 2669 (2824) | 2078 (2267) | 8279 (8916) | 2042 (2404) | 2863 (2948) | 511 |
| `list_100k_medium` (10) | 1528 (1702) | 918 (1081) | not run ³ | 712 (873) | 1339 (1427) | 276 |
| `list_100k_wide` (20) | 2309 (2380) | 1684 (1701) | not run ³ | 1364 (1450) | 2634 (2712) | 387 |

1. Not run: needs a denormalized model, such as orders carrying its
   customer's nation key. MetricFlow joins at most two hops from a measure
   (`MAX_JOIN_HOPS = 2`), and a customer's nation is three hops from a line
   item.
2. Not run: needs a denormalized model, as ¹, and a way to take revenue over
   every nation within a query grouped by nation. As far as we found,
   MetricFlow's metric types compute at the query's own grain.
3. Not run: needs a denormalized model, such as orders carrying its
   customer's attributes. In a query without a metric, as a listing is,
   MetricFlow returns the dimensions one hop from the line item (its order's)
   and the customer's key, but not the customer's dimensions, two hops away
   (observed with 0.214.0; with a metric, as in `by_customer`, it reaches them).

The full notes are in [cases/semantic/metricflow.json](../../cases/semantic/metricflow.json).

## What stands out

### Correctness: every tool that ran a case returned the right numbers

Every run of every case matched its answer, value by value: the aggregates,
`fan_out` included (anfra and Cube each count an order once, though the query
also sums its line items), and every row of every listing, up to 999,052 rows
and 20 columns.

### HTTP, aggregates: anfra and Cube are even, except on the share of total

| Case | anfra HTTP | Cube | Cube ÷ anfra |
|---|---:|---:|---:|
| `total` | 400 | 418 | 1.04× |
| `by_month` | 1411 | 1240 | 0.88× |
| `by_nation` | 1492 | 1520 | 1.02× |
| `fan_out` | 632 | 635 | 1.00× |
| `share_of_total` | 1919 | 2998 | 1.56× |
| `by_customer` | 2412 | 2591 | 1.07× |

- **On most aggregates, the time is Postgres's.** Both tools write SQL close to
  the hand-written query, and take about what Postgres takes to run it.
- **`share_of_total`: Cube's SQL does the joined work twice.** Both tools read
  the line items twice, but Cube computes revenue by nation twice, each with
  the four-table join (line items, orders, customers, nations): once for the
  revenue, once for the inner sum of its multi-stage measure. anfra's total
  over every nation needs no join, so its second pass is a plain sum over the
  line items. On Postgres, by its plans, Cube's SQL took 3.5 s and anfra's
  2.3 s.
- **`by_month`: Cube is 12% faster than anfra HTTP here.** anfra's warm CLI,
  on the same server and SQL, took 1251 ms, as Cube did, and anfra HTTP took
  1216 ms in the replaced run (see Caveats); we have not found why it was
  slower in this one.

### HTTP, listings: Cube is faster on small results, anfra on large and wide ones

| Case | anfra HTTP | Cube | Cube ÷ anfra |
|---|---:|---:|---:|
| `list_1k` (2 columns) | 19 | 8 | 0.44× |
| `list_10k` (2) | 31 | 28 | 0.90× |
| `list_100k` (2) | 209 | 279 | 1.34× |
| `list_1m` (2) | 2042 | 2863 | 1.40× |
| `list_100k_medium` (10) | 712 | 1339 | 1.88× |
| `list_100k_wide` (20) | 1364 | 2634 | 1.93× |

- **Listings are where the tools' own cost shows.** Postgres lists these rows
  in 30–511 ms; the rest of each time is the tool fetching, converting and
  delivering them.
- **Small results: Cube answers 1k rows in 8 ms, anfra in 19 ms.** We checked
  that Cube's runs reach Postgres every time (it runs about two queries a call:
  its refresh-key check, and the listing): its result is not from its cache.
- **Large results: anfra is 1.3–1.4× faster at 100k and 1M rows, and the gap
  grows with width:** 1.9× at 10 and 20 columns. Cube's answer is one JSON
  document of row objects, each value keyed by its member's name; anfra streams
  rows as arrays.

### File: MetricFlow pays about 1.8 s per call before its query

| Case | anfra CLI, warm | anfra CLI, cold | MetricFlow | MetricFlow ÷ anfra cold |
|---|---:|---:|---:|---:|
| `total` | 416 | 1057 | 2209 | 2.1× |
| `by_month` | 1251 | 1842 | 3033 | 1.6× |
| `by_customer` | 2394 | 3004 | 4716 | 1.6× |
| `list_1k` | 21 | 623 | 1788 | 2.9× |
| `list_100k` | 225 | 922 | 2486 | 2.7× |
| `list_1m` | 2078 | 2669 | 8279 | 3.1× |

- **Each `mf query` starts a Python process**, loads dbt and the semantic
  manifest, then queries: about 1.8 s before the query, on every call.
  MetricFlow has no server to keep warm, so the comparison that matches it is
  anfra's cold CLI, which starts its own sidecars on every call (about 0.6 s
  before the query).
- **On large listings, MetricFlow's delivery adds to it:** 1M rows take it
  8.3 s, against 2.7 s for anfra's cold CLI.
- **anfra's warm CLI** forwards each call to `anfra serve`, so it pays neither
  startup.

### Observed: dbt's usage tracking can block each MetricFlow call

dbt sends anonymous usage tracking by default, and it does so **before** each
command, waiting for it, rather than in the background. Every `mf query` pays
it, on top of the times above, unless tracking is turned off
(`send_anonymous_usage_stats: false` in the dbt project, or
`DBT_SEND_ANONYMOUS_USAGE_STATS=false`).

Its cost depends on the network to dbt's tracking endpoint, not on MetricFlow.
On this machine, on the same day, `mf query --metrics revenue` (the `total`
case) took:

| Tracking | One call |
|---|---:|
| off (as in the measured runs) | 2.2 s |
| on, in the morning | 3.5 s |
| on, in the afternoon | 12.5 s |

So it cost from about 1.3 s to about 10 s a call. **The measured runs have
tracking off**, as every other tool's telemetry is: the benchmark compares the
tools' query work, and this cost belongs to the network. A MetricFlow user with
dbt's defaults pays it, and on a slow or restricted network it can be most of
each call's time.

## Caveats

- **Postgres is local**, so each tool's own cost is as visible as it gets.
- **The hand-written SQL column is one run with psql**, including psql's
  connection (about 25 ms), not a measured median.
- **5 runs per case.** The slowest run is within 32% of the median in every
  cell but one: Cube's `list_1k`, whose slowest run took 456 ms against a
  median of 8 ms. The medians are what the comparisons use.
- **Earlier runs of this batch were set aside.** A first attempt, with the 6
  aggregate cases, had Postgres run out of shared memory on some parallel joins
  (its `shm_size` is now 1 GB) and dbt's usage tracking on (now off). A second,
  with the same 6 cases and anfra's AQL compiler v1, was replaced by this run,
  which adds the listing cases and runs compiler v2. Of the 27 cells both runs
  measured, 26 agree within 10%; the exception is anfra HTTP's `by_month`, 16%
  slower in this run (1411 ms against 1216 ms), the cell noted above.
