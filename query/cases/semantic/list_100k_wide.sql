-- list_100k_wide: a listing, not an aggregate: the line items of the orders with keys up to 100,000
-- (about 100k rows), 20 columns from the line item, its order and its customer.
-- line_id is TPC-H's line key (order key, line number) as one number, as every tool's model has it.
SELECT l.l_orderkey::bigint * 10 + l.l_linenumber AS line_id,
       l.l_extendedprice AS extended_price,
       l.l_quantity AS quantity,
       l.l_discount AS discount,
       l.l_shipdate AS ship_date,
       l.l_shipmode AS ship_mode,
       o.o_orderdate AS order_date,
       o.o_orderstatus AS order_status,
       c.c_name AS customer,
       c.c_mktsegment AS market_segment,
       l.l_tax AS tax,
       l.l_returnflag AS return_flag,
       l.l_linestatus AS line_status,
       l.l_receiptdate AS receipt_date,
       o.o_orderpriority AS order_priority,
       o.o_clerk AS clerk,
       o.o_totalprice AS total_price,
       c.c_address AS customer_address,
       c.c_phone AS customer_phone,
       c.c_acctbal AS account_balance
FROM tpch.lineitem l
JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
JOIN tpch.customer c ON c.c_custkey = o.o_custkey
WHERE l.l_orderkey <= 100000
ORDER BY 1
