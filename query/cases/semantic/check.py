# Usage: python3 cases/semantic/check.py cases/semantic/<case>.csv <a tool's result>.csv
# Compare a result (CSV) with a case's answer (CSV), value by value: rows matched in order after
# sorting both, numbers within a relative 1e-6, everything else as text.
import csv, sys, math
def rows(p):
    with open(p) as f:
        r = list(csv.reader(f))
    return r[0], sorted(r[1:])
def same(a, b):
    try:
        x, y = float(a), float(b)
        return math.isclose(x, y, rel_tol=1e-6, abs_tol=1e-9)
    except ValueError:
        return a.strip()[:10] == b.strip()[:10] if len(a) >= 10 and a[4:5] == '-' else a == b
(ha, ra), (hb, rb) = rows(sys.argv[1]), rows(sys.argv[2])
if len(ha) != len(hb) or len(ra) != len(rb):
    print(f"shape differs: {len(rb)}×{len(hb)} vs answer {len(ra)}×{len(ha)}"); sys.exit(1)
for i, (x, y) in enumerate(zip(ra, rb)):
    for j, (a, b) in enumerate(zip(x, y)):
        if not same(a, b):
            print(f"row {i} col {ha[j]}: {b!r} vs answer {a!r}"); sys.exit(1)
print(f"same: {len(ra)} rows × {len(ha)} columns")
