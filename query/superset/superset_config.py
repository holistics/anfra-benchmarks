# Superset, set up to be measured: synchronous SQL Lab, every row returned,
# no caches. Bench only: never use these settings for a real deployment.

SECRET_KEY = "bench-only-not-a-secret"

# Return every row of the largest case (1M) rather than a truncated page.
SQL_MAX_ROW = 2_000_000
DISPLAY_MAX_ROW = 2_000_000

# Run SQL Lab queries in the request (no Celery, no results backend), and let
# the largest case finish.
RESULTS_BACKEND = None
SQLLAB_TIMEOUT = 600
SUPERSET_WEBSERVER_TIMEOUT = 600

# No result caching.
CACHE_CONFIG = {"CACHE_TYPE": "NullCache"}
DATA_CACHE_CONFIG = {"CACHE_TYPE": "NullCache"}

# The runner calls the API with a JWT; no browser session to protect.
WTF_CSRF_ENABLED = False
TALISMAN_ENABLED = False
