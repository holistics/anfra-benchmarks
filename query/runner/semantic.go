package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The semantic group's cases are files in cases/semantic, not code: per case,
// its answer (<case>.csv, from <case>.sql) and each tool's form of it. This
// list only orders them: the aggregates, smallest to largest result, then the
// listings, by rows then by width.
var semanticCaseNames = []string{
	"total", "by_month", "by_nation", "fan_out", "share_of_total", "by_customer",
	"list_1k", "list_10k", "list_100k", "list_1m", "list_100k_medium", "list_100k_wide",
}

// semanticDataset is the anfra dataset the semantic cases' AQL runs on.
const semanticDataset = "tpch"

// semanticCase is a semantic case in each tool's language, the answer every
// tool must return, and the tools that don't run it, each with why.
type semanticCase struct {
	AQL         string
	MFArgs      []string
	CubeQuery   json.RawMessage
	CubeColumns []string
	Answer      [][]string
	NotRun      map[string]string // tool name → what it would need to run the case
}

func loadSemanticCases(bench string) ([]benchCase, error) {
	dir := filepath.Join(bench, "cases", "semantic")
	var mf map[string]struct {
		Args   []string `json:"args"`
		NotRun string   `json:"not_run"`
	}
	var cube map[string]struct {
		Query   json.RawMessage `json:"query"`
		Columns []string        `json:"columns"`
	}
	for name, into := range map[string]any{"metricflow.json": &mf, "cube.json": &cube} {
		if err := readCaseForms(filepath.Join(dir, name), into); err != nil {
			return nil, err
		}
	}
	var cases []benchCase
	for _, name := range semanticCaseNames {
		aql, err := os.ReadFile(filepath.Join(dir, "anfra", name+".aql"))
		if err != nil {
			return nil, err
		}
		answer, err := readCSV(filepath.Join(dir, name+".csv"))
		if err != nil {
			return nil, fmt.Errorf("the answer of %s: %w", name, err)
		}
		sc := &semanticCase{AQL: string(aql), Answer: answer[1:], NotRun: map[string]string{}}
		if m, ok := mf[name]; ok {
			sc.MFArgs = m.Args
			if m.NotRun != "" {
				sc.NotRun["metricflow"] = m.NotRun
			}
		} else {
			sc.NotRun["metricflow"] = "No MetricFlow form in cases/semantic/metricflow.json."
		}
		if c, ok := cube[name]; ok {
			sc.CubeQuery, sc.CubeColumns = c.Query, c.Columns
		} else {
			sc.NotRun["cube"] = "No Cube form in cases/semantic/cube.json."
		}
		cases = append(cases, benchCase{Name: name, Rows: len(answer) - 1, Cols: len(answer[0]), Semantic: sc})
	}
	return cases, nil
}

// readCaseForms reads a tool's forms of the cases, by case name, into into (a
// map), leaving out keys that start with "_", such as "_doc".
func readCaseForms(path string, into any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for key := range all {
		if strings.HasPrefix(key, "_") {
			delete(all, key)
		}
	}
	cases, _ := json.Marshal(all)
	if err := json.Unmarshal(cases, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err == nil && len(rows) == 0 {
		err = fmt.Errorf("%s is empty", path)
	}
	return rows, err
}

// checkValues compares a result with its answer, value by value: rows matched
// after sorting both, numbers within a relative 1e-6 (tools' number types
// differ: anfra keeps 9 decimals, others floats), dates by their day (tools
// write a month as 1992-01-01, 1992-01-01 00:00:00 or 1992-01-01T00:00:00.000),
// anything else as text.
func checkValues(answer, got [][]string) error {
	if len(got) != len(answer) {
		return fmt.Errorf("returned %d rows, want %d", len(got), len(answer))
	}
	want, have := sortedRows(answer), sortedRows(got)
	for i := range want {
		if len(have[i]) != len(want[i]) {
			return fmt.Errorf("row %d has %d values, want %d", i, len(have[i]), len(want[i]))
		}
		for j := range want[i] {
			if !sameValue(want[i][j], have[i][j]) {
				return fmt.Errorf("row %d, column %d: %q, want %q", i, j+1, have[i][j], want[i][j])
			}
		}
	}
	return nil
}

func sortedRows(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = make([]string, len(r))
		for j, v := range r {
			out[i][j] = normalizeDay(v)
		}
	}
	slices.SortFunc(out, func(a, b []string) int { return strings.Compare(strings.Join(a, "\x00"), strings.Join(b, "\x00")) })
	return out
}

// normalizeDay is a date or timestamp's day (2026-01-02), and any other value
// as it is.
func normalizeDay(v string) string {
	if len(v) >= 10 && v[4] == '-' && v[7] == '-' {
		if _, err := time.Parse("2006-01-02", v[:10]); err == nil {
			return v[:10]
		}
	}
	return v
}

func sameValue(want, have string) bool {
	if want == have {
		return true
	}
	w, err1 := strconv.ParseFloat(want, 64)
	h, err2 := strconv.ParseFloat(have, 64)
	if err1 != nil || err2 != nil {
		return false
	}
	return math.Abs(w-h) <= 1e-9 || math.Abs(w-h) <= 1e-6*math.Max(math.Abs(w), math.Abs(h))
}

// textRow is a result row's values as text, as a CSV file would hold them.
func textRow(values []any) []string {
	row := make([]string, len(values))
	for i, v := range values {
		switch v := v.(type) {
		case nil:
			row[i] = ""
		case string:
			row[i] = v
		default:
			row[i] = fmt.Sprint(v)
		}
	}
	return row
}

// --- MetricFlow: the CLI, a new Python process each call. ----------------------

// metricflowTool is `mf query`, in the dbt project (metricflow/), writing the
// result to a CSV file.
func metricflowTool(bench string) tool {
	return cliTool{
		name: "metricflow",
		dir:  filepath.Join(bench, "metricflow"),
		// No usage tracking, as for every other tool (see metricflow/dbt_project.yml).
		env: []string{"DBT_PROFILES_DIR=.", "DBT_SEND_ANONYMOUS_USAGE_STATS=false", "DO_NOT_TRACK=1"},
		argv: func(c benchCase, out string) []string {
			argv := []string{filepath.Join(bench, ".bin", "mf-venv", "bin", "mf"), "query", "--quiet"}
			argv = append(argv, c.Semantic.MFArgs...)
			return append(argv, "--csv", out)
		},
	}
}

// --- Cube: its REST API, as its frontends call it. -----------------------------

// cubeTool is Cube's /cubejs-api/v1/load. Cube answers "Continue wait" while a
// query takes longer than its wait window; the tool asks again, as Cube's own
// client does, and the time counts until the answer is read and parsed.
type cubeTool struct {
	base, secret string
}

func (cubeTool) Name() string { return "cube" }
func (cubeTool) Path() string { return "http" }

// token is a JWT signed with Cube's API secret, as Cube's REST API requires.
func (t cubeTool) token() string {
	enc := base64.RawURLEncoding
	head := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := enc.EncodeToString([]byte(fmt.Sprintf(`{"iat":%d}`, time.Now().Unix())))
	mac := hmac.New(sha256.New, []byte(t.secret))
	mac.Write([]byte(head + "." + claims))
	return head + "." + claims + "." + enc.EncodeToString(mac.Sum(nil))
}

func (t cubeTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	body, _ := json.Marshal(map[string]json.RawMessage{"query": c.Semantic.CubeQuery})
	token := t.token()

	start := time.Now()
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/cubejs-api/v1/load", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", token)
		resp, err := httpClient.Do(req)
		if err != nil {
			return time.Since(start), shape{}, err
		}
		var answer struct {
			Error string           `json:"error"`
			Data  []map[string]any `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&answer)
		resp.Body.Close()
		if err != nil {
			return time.Since(start), shape{}, fmt.Errorf("HTTP %d, parsing the answer: %w", resp.StatusCode, err)
		}
		if answer.Error == "Continue wait" {
			continue
		}
		took := time.Since(start)
		if answer.Error != "" || resp.StatusCode != http.StatusOK {
			return took, shape{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(answer.Error, 300))
		}
		s := shape{Rows: len(answer.Data), Cols: len(c.Semantic.CubeColumns)}
		for _, row := range answer.Data {
			values := make([]any, len(c.Semantic.CubeColumns))
			for i, member := range c.Semantic.CubeColumns {
				values[i] = row[member]
			}
			s.values = append(s.values, textRow(values))
		}
		return took, s, nil
	}
}
