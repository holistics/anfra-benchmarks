-- bench_wide in ClickHouse: the same 1M rows as Postgres, copied from it, so every warehouse
-- holds exactly the same values. Types match Postgres's; every column but id is nullable.
CREATE TABLE bench.bench_wide
(
  id Int64,
  i1 Nullable(Int32),
  b1 Nullable(Int64),
  n1 Nullable(Decimal(18, 4)),
  f1 Nullable(Float64),
  s1 Nullable(String),
  l1 Nullable(String),
  d1 Nullable(Date),
  t1 Nullable(DateTime64(6)),
  z1 Nullable(DateTime64(6, 'UTC')),
  x1 Nullable(Bool),
  i2 Nullable(Int32),
  b2 Nullable(Int64),
  n2 Nullable(Decimal(18, 4)),
  f2 Nullable(Float64),
  s2 Nullable(String),
  l2 Nullable(String),
  d2 Nullable(Date),
  t2 Nullable(DateTime64(6)),
  z2 Nullable(DateTime64(6, 'UTC')),
  x2 Nullable(Bool),
  i3 Nullable(Int32),
  b3 Nullable(Int64),
  n3 Nullable(Decimal(18, 4)),
  f3 Nullable(Float64),
  s3 Nullable(String),
  l3 Nullable(String),
  d3 Nullable(Date),
  t3 Nullable(DateTime64(6)),
  z3 Nullable(DateTime64(6, 'UTC')),
  x3 Nullable(Bool),
  i4 Nullable(Int32),
  b4 Nullable(Int64),
  n4 Nullable(Decimal(18, 4)),
  f4 Nullable(Float64),
  s4 Nullable(String),
  l4 Nullable(String),
  d4 Nullable(Date),
  t4 Nullable(DateTime64(6)),
  z4 Nullable(DateTime64(6, 'UTC')),
  x4 Nullable(Bool),
  i5 Nullable(Int32),
  b5 Nullable(Int64),
  n5 Nullable(Decimal(18, 4)),
  f5 Nullable(Float64),
  s5 Nullable(String),
  l5 Nullable(String),
  d5 Nullable(Date),
  t5 Nullable(DateTime64(6)),
  z5 Nullable(DateTime64(6, 'UTC')),
  x5 Nullable(Bool)
)
ENGINE = MergeTree
ORDER BY id;

INSERT INTO bench.bench_wide
SELECT * FROM postgresql('postgres:5432', 'bench', 'bench_wide', 'bench', 'bench');

OPTIMIZE TABLE bench.bench_wide FINAL;
