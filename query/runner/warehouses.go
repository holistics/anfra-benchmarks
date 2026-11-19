package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A warehouse is where the cases run, and how each tool reaches it: its native
// CLI (the floor), usql's DSN, the name every other tool knows it by (the anfra
// data source, and the Superset and Metabase database), and the table the cases
// scan.
type warehouse struct {
	Name     string
	Source   string
	USQLDSN  string
	Table    string
	nativeCL func(bench string) tool
}

// loadWarehouse is the warehouse by its name. Snowflake's connection is read
// from .bin/snowflake.json, which snowflake/setup.py writes from the bench_sf
// data source, so no credential is in this code.
func loadWarehouse(name, bench string) (warehouse, error) {
	switch name {
	case "postgres":
		return warehouse{
			Name:     "postgres",
			Source:   "bench_pg",
			USQLDSN:  "postgres://bench:bench@127.0.0.1:55432/bench?sslmode=disable",
			Table:    "bench_wide",
			nativeCL: func(string) tool { return psqlTool() },
		}, nil
	case "clickhouse":
		return warehouse{
			Name:     "clickhouse",
			Source:   "bench_ch",
			USQLDSN:  "clickhouse://bench:bench@127.0.0.1:59000/bench",
			Table:    "bench_wide",
			nativeCL: func(bench string) tool { return clickhouseClientTool(filepath.Join(bench, ".bin", "clickhouse")) },
		}, nil
	case "snowflake":
		var conn struct {
			USQLDSN string `json:"usql_dsn"`
			Table   string `json:"table"`
		}
		b, err := os.ReadFile(filepath.Join(bench, ".bin", "snowflake.json"))
		if err != nil {
			return warehouse{}, fmt.Errorf("reading Snowflake's connection: %w; run snowflake/setup.py first", err)
		}
		if err := json.Unmarshal(b, &conn); err != nil {
			return warehouse{}, fmt.Errorf("reading .bin/snowflake.json: %w", err)
		}
		return warehouse{
			Name:     "snowflake",
			Source:   "bench_sf",
			USQLDSN:  conn.USQLDSN,
			Table:    conn.Table,
			nativeCL: func(bench string) tool { return snowTool(filepath.Join(bench, ".bin")) },
		}, nil
	}
	return warehouse{}, fmt.Errorf("no warehouse %q: use postgres, clickhouse or snowflake", name)
}

// on is the case as it runs on w: the same query, on w's table.
func (c benchCase) on(w warehouse) benchCase {
	c.SQL = strings.Replace(c.SQL, "FROM bench_wide", "FROM "+w.Table, 1)
	return c
}
