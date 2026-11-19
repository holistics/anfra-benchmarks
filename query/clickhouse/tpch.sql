-- TPC-H's tables in ClickHouse, database tpch, for the semantic group: the same rows as Postgres's
-- (postgres/tpch.sh), copied from it, so every warehouse holds exactly the same values and the
-- answers in cases/semantic apply to both. Types match Postgres's. Each table is a MergeTree
-- ordered by its primary key. Run it with clickhouse/tpch.sh.
CREATE DATABASE IF NOT EXISTS tpch;

CREATE OR REPLACE TABLE tpch.region (r_regionkey Int32, r_name String, r_comment Nullable(String))
ENGINE = MergeTree ORDER BY r_regionkey;
CREATE OR REPLACE TABLE tpch.nation (n_nationkey Int32, n_name String, n_regionkey Int32, n_comment Nullable(String))
ENGINE = MergeTree ORDER BY n_nationkey;
CREATE OR REPLACE TABLE tpch.customer (
  c_custkey Int32, c_name String, c_address String, c_nationkey Int32, c_phone String,
  c_acctbal Decimal(15, 2), c_mktsegment String, c_comment String
) ENGINE = MergeTree ORDER BY c_custkey;
CREATE OR REPLACE TABLE tpch.orders (
  o_orderkey Int32, o_custkey Int32, o_orderstatus String, o_totalprice Decimal(15, 2), o_orderdate Date,
  o_orderpriority String, o_clerk String, o_shippriority Int32, o_comment String
) ENGINE = MergeTree ORDER BY o_orderkey;
CREATE OR REPLACE TABLE tpch.lineitem (
  l_orderkey Int32, l_partkey Int32, l_suppkey Int32, l_linenumber Int32,
  l_quantity Decimal(15, 2), l_extendedprice Decimal(15, 2), l_discount Decimal(15, 2), l_tax Decimal(15, 2),
  l_returnflag String, l_linestatus String, l_shipdate Date, l_commitdate Date, l_receiptdate Date,
  l_shipinstruct String, l_shipmode String, l_comment String
) ENGINE = MergeTree ORDER BY (l_orderkey, l_linenumber);
