package main

import "path/filepath"

// A warehouse is where the cases run, and how each tool reaches it: its native
// CLI (the floor), usql's DSN, and the name every other tool knows it by (the
// anfra data source, and the Superset and Metabase database).
type warehouse struct {
	Name     string
	Source   string
	USQLDSN  string
	nativeCL func(bench string) tool
}

var warehouses = map[string]warehouse{
	"postgres": {
		Name:     "postgres",
		Source:   "bench_pg",
		USQLDSN:  "postgres://bench:bench@127.0.0.1:55432/bench?sslmode=disable",
		nativeCL: func(string) tool { return psqlTool() },
	},
	"clickhouse": {
		Name:     "clickhouse",
		Source:   "bench_ch",
		USQLDSN:  "clickhouse://bench:bench@127.0.0.1:59000/bench",
		nativeCL: func(bench string) tool { return clickhouseClientTool(filepath.Join(bench, ".bin", "clickhouse")) },
	},
}
