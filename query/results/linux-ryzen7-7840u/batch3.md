# Batch 3: ClickHouse, SQL group, every tool

ClickHouse × clickhouse-client, usql, anfra (CLI cold, CLI warm, HTTP),
Superset, Metabase. SQL only. The same tools and cases as Postgres's
[batch 2](batch2.md), on ClickHouse. Raw runs: [batch3.jsonl](batch3.jsonl).

- **Run:** 2026-11-18. 1 warm-up round, then 5 measured rounds, tools and
  cases shuffled every round. 315 measured runs, all with the expected rows and
  columns.
- **Versions:** anfra 0.22.0, ClickHouse 26.8.39.19 (server and client),
  Superset 6.1.0 with clickhouse-connect 1.13.0, Metabase v0.65.5.1 (built-in
  driver), usql v0.21.6 built with its ClickHouse driver.
- **Data:** `bench_wide` copied from the Postgres one, checked to hold the same
  values (row count, NULL counts, column sums).
- **Machine:** AMD Ryzen 7 PRO 7840U (16 threads), 27 GiB, Linux 7.0. Not
  isolated: other containers were running, idle.
- **How each tool is run:** see [IMPLEMENTATION.md](../../IMPLEMENTATION.md).
  On the file path, every tool writes the whole result to a CSV file.

## Results

Median ms, with the slowest of the 5 runs in brackets. Compare only within a
path.

| Case | clickhouse-client | usql | anfra CLI, cold | anfra CLI, warm | anfra HTTP | Superset | Metabase |
|---|---:|---:|---:|---:|---:|---:|---:|
| | *file* | *file* | *file* | *file* | *http* | *http* | *http* |
| `select1` | 58.1 (77.3) | 16.8 (17.9) | 153 (161) | 12.0 (12.8) | 3.8 (3.8) | 72.1 (91.7) | 20.6 (28.0) |
| `narrow_1` | 54.9 (58.1) | 19.0 (31.6) | 151 (164) | 12.9 (14.6) | 4.9 (6.2) | 74.2 (76.3) | 21.1 (26.9) |
| `narrow_1k` | 58.5 (75.1) | 30.8 (33.1) | 153 (164) | 16.6 (22.2) | 8.5 (11.9) | 75.0 (105) | 22.7 (35.0) |
| `narrow_100k` | 63.4 (76.9) | 967 (984) | 363 (406) | 198 (209) | 188 (192) | 760 (809) | 438 (550) |
| `narrow_1m` | 88.0 (106) | 8969 (9142) | 2114 (2186) | 1903 (2584) | 1766 (2320) | 7162 (7836) | 4092 (4356) |
| `medium_100k` | 89.9 (95.8) | 2973 (3041) | 687 (887) | 528 (543) | 375 (386) | 3298 (3440) | 1887 (2751) |
| `wide_1` | 63.3 (73.1) | 27.6 (29.9) | 159 (170) | 21.6 (23.2) | 12.8 (16.2) | 86.1 (228) | 34.0 (36.8) |
| `wide_1k` | 75.1 (85.0) | 125 (145) | 198 (220) | 41.1 (44.2) | 35.1 (43.9) | 165 (179) | 93.5 (103) |
| `wide_100k` | 158 (180) | 8799 (10.1s) | 1900 (1974) | 1680 (2044) | 1169 (1353) | 9838 (10.4s) | 5667 (8390) |

## What stands out

### File: clickhouse-client is 3–22× faster than anfra on large results

| Case | clickhouse-client | anfra CLI, warm | anfra ÷ clickhouse-client |
|---|---:|---:|---:|
| `select1` | 58.1 | 12.0 | 0.21× |
| `narrow_1k` | 58.5 | 16.6 | 0.28× |
| `wide_1k` | 75.1 | 41.1 | 0.55× |
| `narrow_100k` | 63.4 | 198 | 3.1× |
| `medium_100k` | 89.9 | 528 | 5.9× |
| `wide_100k` | 158 | 1680 | 11× |
| `narrow_1m` | 88.0 | 1903 | 22× |

- **clickhouse-client pays about 55 ms to start**, every call: it is one large
  binary. On small results anfra's warm CLI is ahead of it, at 12–17 ms.
- **On large results it is the fastest tool measured so far**, on any path:
  1M rows in 88 ms. It reads ClickHouse's native columnar format and writes the
  CSV itself, with almost nothing in between. psql, on Postgres, took 595 ms.
- **anfra takes the same time on ClickHouse as on Postgres** for large results
  (`narrow_1m`: 1903 ms here, 1946 ms in batch 2; `wide_100k`: 1680 and 1692).
  The faster warehouse saves anfra nothing, so its time on large results is
  spent in anfra's own path: fetching, converting and writing the rows. That
  is the gap to close.

### HTTP: anfra is 2–5× faster than Metabase, and 4–19× faster than Superset

| Case | anfra HTTP | Metabase | Superset | Metabase ÷ anfra | Superset ÷ anfra |
|---|---:|---:|---:|---:|---:|
| `select1` | 3.8 | 20.6 | 72.1 | 5.4× | 19× |
| `narrow_1k` | 8.5 | 22.7 | 75.0 | 2.7× | 8.8× |
| `narrow_100k` | 188 | 438 | 760 | 2.3× | 4.0× |
| `narrow_1m` | 1766 | 4092 | 7162 | 2.3× | 4.1× |
| `medium_100k` | 375 | 1887 | 3298 | 5.0× | 8.8× |
| `wide_1k` | 35.1 | 93.5 | 165 | 2.7× | 4.7× |
| `wide_100k` | 1169 | 5667 | 9838 | 4.8× | 8.4× |

The order is the same as on Postgres: anfra, then Metabase, then Superset.
Large results take each of them about as long as on Postgres. Metabase is about
30% slower than on Postgres on the mixed and wide cases.

### Against Postgres (batch 2)

- **Small results cost anfra more on ClickHouse:** `select1` takes 3.8 ms over
  HTTP, against 1.6 ms on Postgres. A ClickHouse round trip costs more than a
  Postgres one on a pooled connection. Still well ahead of every other HTTP tool.
- **Large results cost every tool but the native client the same** on both
  warehouses, within about 10%, Metabase aside. These tools spend their time on
  the rows after the warehouse has sent them.

## Caveats

- **ClickHouse is local**, as Postgres was, so each tool's own cost is as
  visible as it gets.
- **This batch was noisier than batch 2.** For most cells over 100 ms the
  slowest run is within 25% of the median, but a few are not: Metabase on
  `wide_100k` and `medium_100k` (48% and 46%), anfra on `narrow_1m` (36% CLI,
  31% HTTP). The first round ran 10 s slower than the others. The medians are
  what the comparisons above use.
- **A ClickHouse scan with LIMIT and no ORDER BY** may return other rows than
  Postgres's first ones. The rows' shape, which the check compares, is the same.
