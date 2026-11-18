-- bench_wide: 1M rows, 50 typed columns, about 10% NULLs in every column but id.
-- Values are derived from id, so every load produces the same table.
--
-- Columns come in 5 groups of 10, one of each type (g = 1..5):
--   i<g> integer, b<g> bigint, n<g> numeric(18,4), f<g> double precision,
--   s<g> short text (12 chars), l<g> long text (96 chars), d<g> date,
--   t<g> timestamp, z<g> timestamptz, x<g> boolean
-- Text is hex, so it holds no comma, quote or newline.

DO $$
DECLARE
  cols text := '';
  exprs text := '';
  g int;
  h text;
BEGIN
  FOR g IN 1..5 LOOP
    -- A per-group hash of id: g picks another multiplier, so groups differ.
    h := format('((id * %s + %s) %% 1000003)', 7919 * g, 104729 * g);
    cols := cols || format(
      ', i%1$s integer, b%1$s bigint, n%1$s numeric(18,4), f%1$s double precision'
      ', s%1$s text, l%1$s text, d%1$s date, t%1$s timestamp, z%1$s timestamptz, x%1$s boolean', g);
    exprs := exprs || format(
      ', CASE WHEN %1$s %% 10 = 0 THEN NULL ELSE %1$s::integer END'
      ', CASE WHEN %1$s %% 10 = 1 THEN NULL ELSE %1$s::bigint * 1000003 END'
      ', CASE WHEN %1$s %% 10 = 2 THEN NULL ELSE (%1$s::numeric / 7) END'
      ', CASE WHEN %1$s %% 10 = 3 THEN NULL ELSE %1$s::double precision / 3 END'
      ', CASE WHEN %1$s %% 10 = 4 THEN NULL ELSE left(md5(%1$s::text), 12) END'
      ', CASE WHEN %1$s %% 10 = 5 THEN NULL ELSE repeat(md5(%1$s::text), 3) END'
      ', CASE WHEN %1$s %% 10 = 6 THEN NULL ELSE date ''2000-01-01'' + (%1$s %% 10000)::int END'
      ', CASE WHEN %1$s %% 10 = 7 THEN NULL ELSE timestamp ''2000-01-01'' + %1$s * interval ''997 seconds'' END'
      ', CASE WHEN %1$s %% 10 = 8 THEN NULL ELSE timestamptz ''2000-01-01 00:00:00+00'' + %1$s * interval ''991 seconds'' END'
      ', CASE WHEN %1$s %% 10 = 9 THEN NULL ELSE %1$s %% 2 = 0 END', h);
  END LOOP;

  EXECUTE 'CREATE TABLE bench_wide (id bigint PRIMARY KEY' || cols || ')';
  EXECUTE 'INSERT INTO bench_wide SELECT id' || exprs || ' FROM generate_series(1::bigint, 1000000) AS id';
END $$;

VACUUM ANALYZE bench_wide;
