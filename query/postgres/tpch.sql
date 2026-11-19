-- TPC-H's tables in the benchmark Postgres, schema tpch, for the semantic group. The standard
-- TPC-H definitions, with the primary keys the spec defines and no other index, so every tool's
-- SQL runs on the same tables. postgres/tpch.sh creates them, loads them, then analyses them.

DROP SCHEMA IF EXISTS tpch CASCADE;
CREATE SCHEMA tpch;

CREATE TABLE tpch.region (
  r_regionkey integer PRIMARY KEY,
  r_name      varchar(25) NOT NULL,
  r_comment   varchar(152)
);

CREATE TABLE tpch.nation (
  n_nationkey integer PRIMARY KEY,
  n_name      varchar(25) NOT NULL,
  n_regionkey integer NOT NULL,
  n_comment   varchar(152)
);

CREATE TABLE tpch.customer (
  c_custkey    integer PRIMARY KEY,
  c_name       varchar(25) NOT NULL,
  c_address    varchar(40) NOT NULL,
  c_nationkey  integer NOT NULL,
  c_phone      varchar(15) NOT NULL,
  c_acctbal    numeric(15, 2) NOT NULL,
  c_mktsegment varchar(10) NOT NULL,
  c_comment    varchar(117) NOT NULL
);

CREATE TABLE tpch.orders (
  o_orderkey      integer PRIMARY KEY,
  o_custkey       integer NOT NULL,
  o_orderstatus   char(1) NOT NULL,
  o_totalprice    numeric(15, 2) NOT NULL,
  o_orderdate     date NOT NULL,
  o_orderpriority varchar(15) NOT NULL,
  o_clerk         varchar(15) NOT NULL,
  o_shippriority  integer NOT NULL,
  o_comment       varchar(79) NOT NULL
);

CREATE TABLE tpch.lineitem (
  l_orderkey      integer NOT NULL,
  l_partkey       integer NOT NULL,
  l_suppkey       integer NOT NULL,
  l_linenumber    integer NOT NULL,
  l_quantity      numeric(15, 2) NOT NULL,
  l_extendedprice numeric(15, 2) NOT NULL,
  l_discount      numeric(15, 2) NOT NULL,
  l_tax           numeric(15, 2) NOT NULL,
  l_returnflag    char(1) NOT NULL,
  l_linestatus    char(1) NOT NULL,
  l_shipdate      date NOT NULL,
  l_commitdate    date NOT NULL,
  l_receiptdate   date NOT NULL,
  l_shipinstruct  varchar(25) NOT NULL,
  l_shipmode      varchar(10) NOT NULL,
  l_comment       varchar(44) NOT NULL,
  PRIMARY KEY (l_orderkey, l_linenumber)
);
