# MetricFlow on ClickHouse

The same dbt project as `../metricflow/` (its `dbt_project.yml`, `profiles.yml` and `models/` are
links to it), with its own `target/`: `dbt-metricflow` always reads the semantic manifest from the
project's `target/`, and the manifest names tables in its warehouse's dialect. Every dbt and `mf`
command here runs with `DBT_TARGET=ch`, which `profiles.yml` maps to the benchmark ClickHouse.
