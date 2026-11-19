-- by_month: revenue by the month of the order's date.
SELECT date_trunc('month', o.o_orderdate)::date AS order_month,
       sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue
FROM tpch.lineitem l
JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
GROUP BY 1
ORDER BY 1
