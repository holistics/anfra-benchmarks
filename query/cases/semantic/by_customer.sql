-- by_customer: revenue by customer (its name, which is unique), for every customer with an order.
SELECT c.c_name AS customer,
       sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue
FROM tpch.lineitem l
JOIN tpch.orders o ON o.o_orderkey = l.l_orderkey
JOIN tpch.customer c ON c.c_custkey = o.o_custkey
GROUP BY 1
ORDER BY 1
