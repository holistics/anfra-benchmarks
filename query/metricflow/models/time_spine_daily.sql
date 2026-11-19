-- MetricFlow's time spine: one row per day, over TPC-H's dates, in each warehouse's dialect.
{{ config(materialized='table') }}
{% if target.type == 'clickhouse' %}
select toDate('1992-01-01') + number as date_day
from numbers(toUInt64(dateDiff('day', toDate('1992-01-01'), toDate('1999-12-31')) + 1))
{% else %}
select d::date as date_day
from generate_series('1992-01-01'::date, '1999-12-31'::date, interval '1 day') as d
{% endif %}
