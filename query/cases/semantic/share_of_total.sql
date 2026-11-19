-- share_of_total: by the customer's nation, the revenue, and that revenue as a share of the total
-- revenue of every nation (a fraction: 0.04 is 4%).
SELECT n.n_name AS nation,
       sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue,
       sum(l.l_extendedprice * (1 - l.l_discount))
         / sum(sum(l.l_extendedprice * (1 - l.l_discount))) OVER () AS revenue_share
FROM tpch.lineitem l
JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
JOIN tpch.customer c ON c.c_custkey = o.o_custkey
JOIN tpch.nation n ON n.n_nationkey = c.c_nationkey
GROUP BY 1
ORDER BY 1
