"""Connect every tool to Snowflake, from the one place its credentials are kept: the bench_sf data
source in anfra-repo/.anfra/data_sources.yml (git-ignored).

Run from the query folder, with the Python of the Snowflake CLI's environment, which has PyYAML:

    .bin/snow-venv/bin/python snowflake/setup.py

It writes, in .bin/ (git-ignored), readable only by you:
- snow/config.toml and snow/bench_key.p8: the connection `bench` for the Snowflake CLI;
- snowflake.json: the connection for the runner (usql's DSN, and the table to query);
and adds a bench_sf database to Superset and to Metabase, unless one exists. Safe to rerun.

Every tool logs in with the data source's private key (key-pair auth), as anfra does, as the same
user with its default role, in the schema snowflake/init.sql creates (ANFRA_BENCH).
"""

import base64
import json
import os
import sys
import urllib.parse
import urllib.error
import urllib.request

import yaml

SCHEMA = "ANFRA_BENCH"
SUPERSET = "http://127.0.0.1:58088"
METABASE = "http://127.0.0.1:58089"


def write_private(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as f:
        f.write(text)
    os.chmod(path, 0o600)


def call(method, url, body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json", **(headers or {})})
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as err:
        sys.exit(f"{method} {url.split('?')[0]}: HTTP {err.code}: {err.read().decode()[:600]}")


def main():
    sources = yaml.safe_load(open("anfra-repo/.anfra/data_sources.yml"))["data_sources"]
    conn = (sources.get("bench_sf") or {}).get("connection") or {}
    for key in ("host", "user", "private_key", "dbname", "warehouse"):
        if not conn.get(key) or str(conn[key]).startswith("<"):
            sys.exit(f"bench_sf has no {key}: fill it in anfra-repo/.anfra/data_sources.yml (see data_sources.yml.example)")
    account = conn["host"].removesuffix(".snowflakecomputing.com")
    user, database, warehouse = conn["user"], conn["dbname"], conn["warehouse"]
    pem = conn["private_key"].strip() + "\n"
    if "BEGIN PRIVATE KEY" not in pem:
        sys.exit("bench_sf's private_key must be an unencrypted PKCS#8 key (-----BEGIN PRIVATE KEY-----)")
    der = base64.b64decode("".join(line for line in pem.splitlines() if not line.startswith("-----")))
    q = urllib.parse.quote

    # The key, as a file, for the Snowflake CLI.
    key_file = os.path.abspath(".bin/snow/bench_key.p8")
    write_private(key_file, pem)

    # The Snowflake CLI's connection.
    toml = "\n".join([
        "[connections.bench]",
        f'account = "{account}"',
        f'user = "{user}"',
        'authenticator = "SNOWFLAKE_JWT"',
        f'private_key_file = "{key_file}"',
        f'database = "{database}"',
        f'schema = "{SCHEMA}"',
        f'warehouse = "{warehouse}"',
        "",
    ])
    write_private(".bin/snow/config.toml", toml)
    print("snow: connection bench written to .bin/snow/config.toml")

    # The runner's: usql's DSN (the key as base64url DER, as gosnowflake takes it), and the table
    # every tool queries.
    usql_dsn = (f"snowflake://{q(user, safe='')}@{account}/{database}/{SCHEMA}"
                f"?warehouse={q(warehouse, safe='')}&authenticator=SNOWFLAKE_JWT"
                f"&privateKey={q(base64.urlsafe_b64encode(der).decode(), safe='')}")
    write_private(".bin/snowflake.json", json.dumps({"usql_dsn": usql_dsn, "table": f"{SCHEMA}.bench_wide"}, indent=2))
    print("runner: connection written to .bin/snowflake.json")

    # Superset: a database bench_sf, through snowflake-sqlalchemy, with its key-pair auth.
    token = call("POST", f"{SUPERSET}/api/v1/security/login",
                 {"username": "admin", "password": "admin", "provider": "db", "refresh": False})["access_token"]
    auth = {"Authorization": f"Bearer {token}"}
    names = [d["database_name"] for d in call("GET", f"{SUPERSET}/api/v1/database/", headers=auth)["result"]]
    if "bench_sf" in names:
        print("Superset: database bench_sf exists")
    else:
        uri = f"snowflake://{q(user, safe='')}@{account}/{database}/{SCHEMA}?warehouse={q(warehouse, safe='')}"
        extra = {"auth_method": "keypair", "auth_params": {"privatekey_body": pem, "privatekey_pass": None}}
        created = call("POST", f"{SUPERSET}/api/v1/database/", {
            "database_name": "bench_sf", "sqlalchemy_uri": uri, "expose_in_sqllab": True,
            "masked_encrypted_extra": json.dumps(extra),
        }, auth)
        print(f"Superset: database bench_sf added, id {created['id']}")

    # Metabase: a database bench_sf, with its built-in driver and the key uploaded. No value scans
    # (is_full_sync false).
    session = call("POST", f"{METABASE}/api/session", {"username": "admin@bench.invalid", "password": "bench-Metabase-1"})["id"]
    mb = {"X-Metabase-Session": session}
    names = [d["name"] for d in call("GET", f"{METABASE}/api/database", headers=mb)["data"]]
    if "bench_sf" in names:
        print("Metabase: database bench_sf exists")
    else:
        created = call("POST", f"{METABASE}/api/database", {
            "engine": "snowflake", "name": "bench_sf", "is_full_sync": False, "is_on_demand": False,
            "details": {
                "account": account, "user": user, "use-password": False,
                "private-key-options": "uploaded",
                "private-key-value": "data:application/octet-stream;base64," + base64.b64encode(pem.encode()).decode(),
                "db": database, "warehouse": warehouse,
                "schema-filters-type": "inclusion", "schema-filters-patterns": SCHEMA,
            },
        }, mb)
        print(f"Metabase: database bench_sf added, id {created['id']}")


if __name__ == "__main__":
    main()
