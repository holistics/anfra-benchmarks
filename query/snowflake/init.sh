#!/bin/sh
# Run snowflake/init.sql through anfra, one statement at a time, on the bench repo's bench_sf data
# source (anfra-repo/.anfra/data_sources.yml), so Snowflake is set up as the user and role every tool
# runs as. Statements are split at each ';' that ends a line; init.sql has no other.
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
python3 - "$here/snowflake/init.sql" <<'PY' | while IFS= read -r statement_file; do
import sys, tempfile, re
sql = open(sys.argv[1]).read()
# Drop comment lines, then split at statement ends.
body = "\n".join(line for line in sql.splitlines() if not line.lstrip().startswith("--"))
for statement in re.split(r";\s*\n", body):
    statement = statement.strip().rstrip(";")
    if statement:
        f = tempfile.NamedTemporaryFile("w", suffix=".sql", delete=False)
        f.write(statement)
        f.close()
        print(f.name)
PY
  echo "-- $(head -c 70 "$statement_file" | tr '\n' ' ')…"
  (cd "$here/anfra-repo" && anfra --non-interactive query --pretty --lang sql -s bench_sf < "$statement_file")
  rm -f "$statement_file"
done
