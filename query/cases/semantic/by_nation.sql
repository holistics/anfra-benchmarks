-- by_nation: revenue by the nation of the order's customer: lineitem → orders → customer → nation.
SELECT n.n_name AS nation,
       sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue
FROM tpch.lineitem l
JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
JOIN tpch.customer c ON c.c_custkey = o.o_custkey
JOIN tpch.nation n ON n.n_nationkey = c.c_nationkey
GROUP BY 1
ORDER BY 1
