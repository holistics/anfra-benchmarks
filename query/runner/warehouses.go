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
	// The semantic group's: anfra's dataset, MetricFlow's dbt project folder and
	// target, and Cube's base URL, on this warehouse. Empty where the warehouse
	// has no semantic group yet.
	Dataset     string
	MFProject   string
	MFTarget    string
	CubeBaseURL string
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

			Dataset:     "tpch",
			MFProject:   "metricflow",
			MFTarget:    "bench",
			CubeBaseURL: "http://127.0.0.1:58100",
		}, nil
	case "clickhouse":
		return warehouse{
			Name:     "clickhouse",
			Source:   "bench_ch",
			USQLDSN:  "clickhouse://bench:bench@127.0.0.1:59000/bench",
			Table:    "bench_wide",
			nativeCL: func(bench string) tool { return clickhouseClientTool(filepath.Join(bench, ".bin", "clickhouse")) },

			Dataset:     "tpch_ch",
			MFProject:   "metricflow-ch",
			MFTarget:    "ch",
			CubeBaseURL: "http://127.0.0.1:58101",
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
