-- bench_wide in Snowflake: the same 1M rows as postgres/init.sql, generated here from the same
-- formulas rather than uploaded. Every value is derived from id: each group of 10 columns (g = 1..5)
-- hashes id its own way, h = (id * 7919g + 104729g) % 1000003, and every column is NULL when
-- h % 10 is its place in the group (about 10% NULLs).
--
-- Types match Postgres's: integer, bigint, numeric(18,4), double, short text (12 chars), long text
-- (96 chars), date, timestamp, timestamptz (in UTC), boolean.
--
-- It creates a schema, ANFRA_BENCH, in the data source's database, and the table in it; it creates
-- no database. Run it with snowflake/init.sh, which sends each statement through anfra's bench_sf
-- data source, as the user and role every tool runs as. Each statement stands alone (anfra may run
-- each on its own session), so every name is qualified.

CREATE SCHEMA IF NOT EXISTS ANFRA_BENCH;

CREATE OR REPLACE TABLE ANFRA_BENCH.bench_wide (
  id BIGINT NOT NULL PRIMARY KEY,
  i1 INTEGER,
  b1 BIGINT,
  n1 NUMBER(18, 4),
  f1 DOUBLE,
  s1 VARCHAR,
  l1 VARCHAR,
  d1 DATE,
  t1 TIMESTAMP_NTZ(6),
  z1 TIMESTAMP_TZ(6),
  x1 BOOLEAN,
  i2 INTEGER,
  b2 BIGINT,
  n2 NUMBER(18, 4),
  f2 DOUBLE,
  s2 VARCHAR,
  l2 VARCHAR,
  d2 DATE,
  t2 TIMESTAMP_NTZ(6),
  z2 TIMESTAMP_TZ(6),
  x2 BOOLEAN,
  i3 INTEGER,
  b3 BIGINT,
  n3 NUMBER(18, 4),
  f3 DOUBLE,
  s3 VARCHAR,
  l3 VARCHAR,
  d3 DATE,
  t3 TIMESTAMP_NTZ(6),
  z3 TIMESTAMP_TZ(6),
  x3 BOOLEAN,
  i4 INTEGER,
  b4 BIGINT,
  n4 NUMBER(18, 4),
  f4 DOUBLE,
  s4 VARCHAR,
  l4 VARCHAR,
  d4 DATE,
  t4 TIMESTAMP_NTZ(6),
  z4 TIMESTAMP_TZ(6),
  x4 BOOLEAN,
  i5 INTEGER,
  b5 BIGINT,
  n5 NUMBER(18, 4),
  f5 DOUBLE,
  s5 VARCHAR,
  l5 VARCHAR,
  d5 DATE,
  t5 TIMESTAMP_NTZ(6),
  z5 TIMESTAMP_TZ(6),
  x5 BOOLEAN
);

-- Inserted in id order, so a scan with LIMIT reads the first ids first, as in Postgres.
INSERT INTO ANFRA_BENCH.bench_wide
WITH ids AS (
  SELECT ROW_NUMBER() OVER (ORDER BY SEQ8()) AS id
  FROM TABLE(GENERATOR(ROWCOUNT => 1000000))
),
hashes AS (
  SELECT
    id,
    (id * 7919 + 104729) % 1000003 AS h1,
    (id * 15838 + 209458) % 1000003 AS h2,
    (id * 23757 + 314187) % 1000003 AS h3,
    (id * 31676 + 418916) % 1000003 AS h4,
    (id * 39595 + 523645) % 1000003 AS h5
  FROM ids
)
SELECT
  id,
  CASE WHEN h1 % 10 = 0 THEN NULL ELSE h1::INTEGER END AS i1,
  CASE WHEN h1 % 10 = 1 THEN NULL ELSE h1 * 1000003 END AS b1,
  CASE WHEN h1 % 10 = 2 THEN NULL ELSE CAST(CAST(h1 AS NUMBER(38, 12)) / 7 AS NUMBER(18, 4)) END AS n1,
  CASE WHEN h1 % 10 = 3 THEN NULL ELSE h1::DOUBLE / 3 END AS f1,
  CASE WHEN h1 % 10 = 4 THEN NULL ELSE LEFT(MD5(TO_VARCHAR(h1)), 12) END AS s1,
  CASE WHEN h1 % 10 = 5 THEN NULL ELSE REPEAT(MD5(TO_VARCHAR(h1)), 3) END AS l1,
  CASE WHEN h1 % 10 = 6 THEN NULL ELSE DATEADD(day, h1 % 10000, '2000-01-01'::DATE) END AS d1,
  CASE WHEN h1 % 10 = 7 THEN NULL ELSE DATEADD(second, h1 * 997, '2000-01-01 00:00:00'::TIMESTAMP_NTZ(6)) END AS t1,
  CASE WHEN h1 % 10 = 8 THEN NULL ELSE DATEADD(second, h1 * 991, '2000-01-01 00:00:00 +00:00'::TIMESTAMP_TZ(6)) END AS z1,
  CASE WHEN h1 % 10 = 9 THEN NULL ELSE h1 % 2 = 0 END AS x1,
  CASE WHEN h2 % 10 = 0 THEN NULL ELSE h2::INTEGER END AS i2,
  CASE WHEN h2 % 10 = 1 THEN NULL ELSE h2 * 1000003 END AS b2,
  CASE WHEN h2 % 10 = 2 THEN NULL ELSE CAST(CAST(h2 AS NUMBER(38, 12)) / 7 AS NUMBER(18, 4)) END AS n2,
  CASE WHEN h2 % 10 = 3 THEN NULL ELSE h2::DOUBLE / 3 END AS f2,
  CASE WHEN h2 % 10 = 4 THEN NULL ELSE LEFT(MD5(TO_VARCHAR(h2)), 12) END AS s2,
  CASE WHEN h2 % 10 = 5 THEN NULL ELSE REPEAT(MD5(TO_VARCHAR(h2)), 3) END AS l2,
  CASE WHEN h2 % 10 = 6 THEN NULL ELSE DATEADD(day, h2 % 10000, '2000-01-01'::DATE) END AS d2,
  CASE WHEN h2 % 10 = 7 THEN NULL ELSE DATEADD(second, h2 * 997, '2000-01-01 00:00:00'::TIMESTAMP_NTZ(6)) END AS t2,
  CASE WHEN h2 % 10 = 8 THEN NULL ELSE DATEADD(second, h2 * 991, '2000-01-01 00:00:00 +00:00'::TIMESTAMP_TZ(6)) END AS z2,
  CASE WHEN h2 % 10 = 9 THEN NULL ELSE h2 % 2 = 0 END AS x2,
  CASE WHEN h3 % 10 = 0 THEN NULL ELSE h3::INTEGER END AS i3,
  CASE WHEN h3 % 10 = 1 THEN NULL ELSE h3 * 1000003 END AS b3,
  CASE WHEN h3 % 10 = 2 THEN NULL ELSE CAST(CAST(h3 AS NUMBER(38, 12)) / 7 AS NUMBER(18, 4)) END AS n3,
  CASE WHEN h3 % 10 = 3 THEN NULL ELSE h3::DOUBLE / 3 END AS f3,
  CASE WHEN h3 % 10 = 4 THEN NULL ELSE LEFT(MD5(TO_VARCHAR(h3)), 12) END AS s3,
  CASE WHEN h3 % 10 = 5 THEN NULL ELSE REPEAT(MD5(TO_VARCHAR(h3)), 3) END AS l3,
  CASE WHEN h3 % 10 = 6 THEN NULL ELSE DATEADD(day, h3 % 10000, '2000-01-01'::DATE) END AS d3,
  CASE WHEN h3 % 10 = 7 THEN NULL ELSE DATEADD(second, h3 * 997, '2000-01-01 00:00:00'::TIMESTAMP_NTZ(6)) END AS t3,
  CASE WHEN h3 % 10 = 8 THEN NULL ELSE DATEADD(second, h3 * 991, '2000-01-01 00:00:00 +00:00'::TIMESTAMP_TZ(6)) END AS z3,
  CASE WHEN h3 % 10 = 9 THEN NULL ELSE h3 % 2 = 0 END AS x3,
  CASE WHEN h4 % 10 = 0 THEN NULL ELSE h4::INTEGER END AS i4,
  CASE WHEN h4 % 10 = 1 THEN NULL ELSE h4 * 1000003 END AS b4,
  CASE WHEN h4 % 10 = 2 THEN NULL ELSE CAST(CAST(h4 AS NUMBER(38, 12)) / 7 AS NUMBER(18, 4)) END AS n4,
  CASE WHEN h4 % 10 = 3 THEN NULL ELSE h4::DOUBLE / 3 END AS f4,
  CASE WHEN h4 % 10 = 4 THEN NULL ELSE LEFT(MD5(TO_VARCHAR(h4)), 12) END AS s4,
  CASE WHEN h4 % 10 = 5 THEN NULL ELSE REPEAT(MD5(TO_VARCHAR(h4)), 3) END AS l4,
  CASE WHEN h4 % 10 = 6 THEN NULL ELSE DATEADD(day, h4 % 10000, '2000-01-01'::DATE) END AS d4,
  CASE WHEN h4 % 10 = 7 THEN NULL ELSE DATEADD(second, h4 * 997, '2000-01-01 00:00:00'::TIMESTAMP_NTZ(6)) END AS t4,
  CASE WHEN h4 % 10 = 8 THEN NULL ELSE DATEADD(second, h4 * 991, '2000-01-01 00:00:00 +00:00'::TIMESTAMP_TZ(6)) END AS z4,
  CASE WHEN h4 % 10 = 9 THEN NULL ELSE h4 % 2 = 0 END AS x4,
  CASE WHEN h5 % 10 = 0 THEN NULL ELSE h5::INTEGER END AS i5,
  CASE WHEN h5 % 10 = 1 THEN NULL ELSE h5 * 1000003 END AS b5,
  CASE WHEN h5 % 10 = 2 THEN NULL ELSE CAST(CAST(h5 AS NUMBER(38, 12)) / 7 AS NUMBER(18, 4)) END AS n5,
  CASE WHEN h5 % 10 = 3 THEN NULL ELSE h5::DOUBLE / 3 END AS f5,
  CASE WHEN h5 % 10 = 4 THEN NULL ELSE LEFT(MD5(TO_VARCHAR(h5)), 12) END AS s5,
  CASE WHEN h5 % 10 = 5 THEN NULL ELSE REPEAT(MD5(TO_VARCHAR(h5)), 3) END AS l5,
  CASE WHEN h5 % 10 = 6 THEN NULL ELSE DATEADD(day, h5 % 10000, '2000-01-01'::DATE) END AS d5,
  CASE WHEN h5 % 10 = 7 THEN NULL ELSE DATEADD(second, h5 * 997, '2000-01-01 00:00:00'::TIMESTAMP_NTZ(6)) END AS t5,
  CASE WHEN h5 % 10 = 8 THEN NULL ELSE DATEADD(second, h5 * 991, '2000-01-01 00:00:00 +00:00'::TIMESTAMP_TZ(6)) END AS z5,
  CASE WHEN h5 % 10 = 9 THEN NULL ELSE h5 % 2 = 0 END AS x5
FROM hashes
ORDER BY id;

-- The check: every value below must match Postgres's, which postgres/init.sql produced:
--   rows 1000000 | n1 nulls 100001 | sum(i1) 450001806383 | sum(b2) 450002669145957426
--   | sum(n3) 64285846958.7143 | round(sum(f4), 3) 150000512761 (a sum of doubles: its last
--   decimals vary with the order rows are added, in Postgres as here) | distinct s5 900000
--   | sum(length(l1)) 86400000 | min(d2) 2000-01-01 | max(t3) 2031-08-05 08:59:54
--   | min(z4) 2000-01-01 00:00:00 +00:00 | x5 true 500001
SELECT
  COUNT(*) AS rows_,
  COUNT_IF(n1 IS NULL) AS n1_nulls,
  SUM(i1) AS sum_i1,
  SUM(b2) AS sum_b2,
  SUM(n3) AS sum_n3,
  ROUND(SUM(f4), 3) AS sum_f4,
  COUNT(DISTINCT s5) AS distinct_s5,
  SUM(LENGTH(l1)) AS sum_len_l1,
  MIN(d2) AS min_d2,
  MAX(t3) AS max_t3,
  MIN(z4) AS min_z4,
  COUNT_IF(x5) AS x5_true
FROM ANFRA_BENCH.bench_wide;

-- And two rows, to compare value by value with Postgres's:
--   1      | 112648 | 112648337944 | 16092.5714 | 37549.333333333336 | eaffef894a66 | 2007-04-02
--          | 2003-07-23 21:14:16 | z2 2007-01-28 02:58:56 +00:00 | true
--   500000 | 592852 | 592853778556 | NULL       | 197617.33333333334 | f116653af991 | 2007-10-23
--          | 2018-09-24 03:04:04 | z2 2005-10-30 23:21:31 +00:00 | true
SELECT id, i1, b1, n1, f1, s1, d1, t1, z2, x1 FROM ANFRA_BENCH.bench_wide WHERE id IN (1, 500000) ORDER BY id;
