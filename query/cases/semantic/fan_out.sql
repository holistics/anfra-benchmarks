-- fan_out: by the customer's nation, for customers in the BUILDING market segment, the number of
-- orders (counted on orders: each order once) and the revenue (summed on line items). Joining
-- orders to their line items repeats each order once per line: an order count taken after that
-- join would be about 4 times too high. Each measure is computed at its own grain here.
WITH order_counts AS (
  SELECT n.n_name AS nation, count(*) AS order_count
  FROM tpch.orders o
  JOIN tpch.customer c ON c.c_custkey = o.o_custkey
  JOIN tpch.nation n ON n.n_nationkey = c.c_nationkey
  WHERE c.c_mktsegment = 'BUILDING'
  GROUP BY 1
), revenues AS (
  SELECT n.n_name AS nation, sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue
  FROM tpch.lineitem l
  JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
  JOIN tpch.customer c ON c.c_custkey = o.o_custkey
  JOIN tpch.nation n ON n.n_nationkey = c.c_nationkey
  WHERE c.c_mktsegment = 'BUILDING'
  GROUP BY 1
)
SELECT oc.nation, oc.order_count, r.revenue
FROM order_counts oc
JOIN revenues r ON r.nation = oc.nation
ORDER BY 1
