-- MetricFlow's time spine: one row per day, over TPC-H's dates.
{{ config(materialized='table') }}
select d::date as date_day
from generate_series('1992-01-01'::date, '1999-12-31'::date, interval '1 day') as d
