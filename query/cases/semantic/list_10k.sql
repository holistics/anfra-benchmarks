-- list_10k: a listing, not an aggregate: the line items of the orders with keys up to 10,000
-- (about 10k rows), 2 columns from the line item.
-- line_id is TPC-H's line key (order key, line number) as one number, as every tool's model has it.
SELECT l.l_orderkey::bigint * 10 + l.l_linenumber AS line_id,
       l.l_extendedprice AS extended_price
FROM tpch.lineitem l
WHERE l.l_orderkey <= 10000
ORDER BY 1
