package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// A benchCase is one query every tool runs, and the result shape it must return.
type benchCase struct {
	Name    string   `json:"name"`
	SQL     string   `json:"sql"`
	Rows    int      `json:"rows"`
	Cols    int      `json:"cols"`
	Columns []string `json:"columns"`
	// Semantic is a semantic case's forms and answer; nil for a SQL case.
	Semantic *semanticCase `json:"-"`
}

// narrowColumns are 3 columns of bench_wide: bigint, short text, numeric.
var narrowColumns = []string{"id", "s1", "n1"}

// typedColumns are the first groups of bench_wide's typed columns: 10 a group,
// one of each type.
func typedColumns(groups int) []string {
	var cols []string
	for g := 1; g <= groups; g++ {
		for _, t := range []string{"i", "b", "n", "f", "s", "l", "d", "t", "z", "x"} {
			cols = append(cols, fmt.Sprintf("%s%d", t, g))
		}
	}
	return cols
}

// mediumColumns are 10 columns, one of each type; wideColumns are 30.
var (
	mediumColumns = typedColumns(1)
	wideColumns   = typedColumns(3)
)

func scanCase(name string, cols []string, rows int) benchCase {
	return benchCase{
		Name:    name,
		SQL:     fmt.Sprintf("SELECT %s FROM bench_wide LIMIT %d", strings.Join(cols, ", "), rows),
		Rows:    rows,
		Cols:    len(cols),
		Columns: cols,
	}
}

// allCases are the SQL group's cases: rows × width, plus a query with no data.
var allCases = []benchCase{
	{Name: "select1", SQL: "SELECT 1 AS one", Rows: 1, Cols: 1, Columns: []string{"one"}},
	scanCase("narrow_1", narrowColumns, 1),
	scanCase("narrow_1k", narrowColumns, 1_000),
	scanCase("narrow_100k", narrowColumns, 100_000),
	scanCase("narrow_1m", narrowColumns, 1_000_000),
	scanCase("medium_100k", mediumColumns, 100_000),
	scanCase("wide_1", wideColumns, 1),
	scanCase("wide_1k", wideColumns, 1_000),
	scanCase("wide_100k", wideColumns, 100_000),
}

// writeCases prints every case as a JSON line, with its place in the list, for
// the report to read (cases/cases.jsonl).
func writeCases() error {
	enc := json.NewEncoder(os.Stdout)
	for i, c := range allCases {
		if err := enc.Encode(struct {
			Order int `json:"order"`
			benchCase
		}{i + 1, c}); err != nil {
			return err
		}
	}
	return nil
}
