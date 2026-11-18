# Batch 1: Postgres, SQL group

Postgres × psql, usql, anfra (CLI cold, CLI warm, HTTP), Superset. SQL only.
Raw runs: [batch1.jsonl](batch1.jsonl).

- **Run:** 2026-11-18. 1 warm-up round, then 5 measured rounds, tools and
  cases shuffled every round. 270 measured runs, all with the expected rows and
  columns.
- **Versions:** anfra 0.22.0, Postgres 16, Superset 6.1.0, usql v0.21.6, psql
  15.18.
- **Machine:** AMD Ryzen 7 PRO 7840U (16 threads), 27 GiB, Linux 7.0. Not
  isolated: 13 other containers were running, idle.

## Results

Median ms, with the slowest of the 5 runs in brackets. Compare only within a
path.

| Case | psql | usql | anfra CLI, cold | anfra CLI, warm | anfra HTTP | Superset |
|---|---:|---:|---:|---:|---:|---:|
| | *file* | *file* | *file* | *file* | *http* | *http* |
| `select1` | 27.0 (30.0) | 22.7 (26.2) | 151 (159) | 9.5 (13.0) | 1.7 (1.9) | 59.0 (72.0) |
| `narrow_1` | 30.1 (34.2) | 20.6 (24.5) | 151 (154) | 10.4 (11.4) | 1.8 (2.2) | 59.6 (63.1) |
| `narrow_1k` | 30.0 (33.7) | 33.7 (38.2) | 156 (167) | 14.6 (19.0) | 5.0 (8.5) | 70.9 (84.3) |
| `narrow_100k` | 93.3 (99.9) | 907 (983) | 350 (399) | 218 (230) | 192 (200) | 750 (860) |
| `narrow_1m` | 599 (740) | 8691 (9502) | 2213 (2402) | 2072 (2114) | 1988 (2019) | 7405 (7853) |
| `medium_100k` | 183 (228) | 2835 (3112) | 809 (914) | 668 (697) | 417 (443) | 3105 (3391) |
| `wide_1` | 28.8 (38.9) | 21.5 (22.9) | 151 (155) | 9.9 (11.4) | 2.2 (4.4) | 67.8 (74.8) |
| `wide_1k` | 34.4 (36.1) | 118 (125) | 180 (185) | 29.8 (34.4) | 21.2 (24.2) | 152 (164) |
| `wide_100k` | 428 (495) | 8430 (9291) | 1947 (2146) | 1725 (1965) | 1281 (1407) | 9981 (11.2s) |

Cases: `narrow` is 3 columns, `medium` 10, `wide` 30; the suffix is the rows.
See the [plan](../../README.md#cases).

## What stands out

### HTTP: anfra is 4–8× faster than Superset on every result size

| Case | anfra HTTP | Superset | Superset ÷ anfra |
|---|---:|---:|---:|
| `select1` | 1.7 | 59.0 | 35× |
| `narrow_1k` | 5.0 | 70.9 | 14× |
| `narrow_100k` | 192 | 750 | 3.9× |
| `narrow_1m` | 1988 | 7405 | 3.7× |
| `medium_100k` | 417 | 3105 | 7.4× |
| `wide_100k` | 1281 | 9981 | 7.8× |

Superset's fixed cost is about 60 ms a query; anfra's is about 2 ms. On large
results the gap widens with the number of columns: Superset returns each row as
a JSON object with its column names, and builds it in Python.

### File: anfra is 2–4× psql on large results, and ahead of it on small ones

| Case | psql | anfra CLI, warm | anfra ÷ psql |
|---|---:|---:|---:|
| `select1` | 27.0 | 9.5 | 0.35× |
| `narrow_1k` | 30.0 | 14.6 | 0.5× |
| `narrow_100k` | 93.3 | 218 | 2.3× |
| `medium_100k` | 183 | 668 | 3.7× |
| `narrow_1m` | 599 | 2072 | 3.5× |
| `wide_100k` | 428 | 1725 | 4.0× |

- **Small results:** psql opens a new, password-authenticated connection on
  every call (about 25 ms). anfra's warm server keeps its connections pooled,
  so the warm CLI answers in about 10 ms, a third of psql's time.
- **Large results:** anfra costs about 1.5 µs a row more than psql for 3
  columns, and about 13 µs a row more for 30 columns. That is the main gap to
  close.
- **Cold CLI:** a fixed 140 ms more than warm, for starting the sidecars. It
  matters for one-shot calls, such as an agent's, without `anfra serve`.
- **usql** is 4–5× slower than anfra on large results. It is the slowest tool
  on the file path.

### Where anfra's time goes on large results

`core.query`'s own timings put almost all of it in one step,
`fetching_query_result`: fetching the rows from Postgres, converting them, and
streaming them out.

| Case | anfra total | `fetching_query_result` | Answer size |
|---|---:|---:|---:|
| `narrow_1m` | 2.05 s | 2.03 s | 42 MB |
| `wide_100k` | 1.40 s | 1.40 s | 66 MB |

The HTTP client's parsing is not the cost: curl, which parses nothing, takes
the same time. The next step is breaking that step down with the sidecars'
spans: decoding Postgres rows, converting them to Arrow, and encoding them as
JSON.

## Caveats

- **Postgres is local**, so network time is close to zero and each tool's own
  cost is as visible as it gets. On a remote warehouse, the gaps narrow.
- **5 runs per case.** For every cell over 100 ms, the slowest run is within
  25% of the median, so the medians are stable enough to compare. Cells of a
  few milliseconds vary more in relative terms (up to 2×), but not in ways that
  change any comparison above.
- **The machine was not isolated.** Other containers were idle, but running.
- **Superset** runs one gunicorn worker (the image's default). This batch runs
  one query at a time, so more workers would not change it.
