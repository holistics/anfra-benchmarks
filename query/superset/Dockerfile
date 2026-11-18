FROM apache/superset:6.1.0

USER root
# The release image ships no warehouse drivers.
RUN . /app/.venv/bin/activate && uv pip install --no-cache psycopg2-binary==2.9.10 clickhouse-connect==1.13.0
COPY start.sh /app/bench/start.sh
RUN chmod +x /app/bench/start.sh
USER superset

CMD ["/app/bench/start.sh"]
