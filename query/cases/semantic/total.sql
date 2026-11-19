-- total: the total revenue. Revenue is a line item's extended price, less its discount.
SELECT sum(l.l_extendedprice * (1 - l.l_discount)) AS revenue
FROM tpch.lineitem l
