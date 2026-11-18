package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// A tool runs a case one way (its path: a file from a CLI, or an HTTP answer)
// and says how long that took, and the shape of what it returned.
type tool interface {
	Name() string
	Path() string
	Run(ctx context.Context, c benchCase) (time.Duration, shape, error)
}

// shape is a result's size, which must match its case's.
type shape struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// --- CLI tools: time from start to exit, with the file fully written. -----------

// cliTool runs a command that writes the case's result to a CSV file with a
// header row.
type cliTool struct {
	name string
	dir  string
	env  []string
	argv func(sql, out string) []string
}

func (t cliTool) Name() string { return t.name }
func (t cliTool) Path() string { return "file" }

func (t cliTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	out := filepath.Join(os.TempDir(), fmt.Sprintf("bench-%s-%d.csv", t.name, time.Now().UnixNano()))
	defer os.Remove(out)
	argv := t.argv(c.SQL, out)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = t.dir
	cmd.Env = append(os.Environ(), t.env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	took := time.Since(start)
	if err != nil {
		return took, shape{}, fmt.Errorf("%s: %w: %s", argv[0], err, bytes.TrimSpace(stderr.Bytes()))
	}
	s, err := csvShape(out)
	return took, s, err
}

// csvShape counts a CSV file's columns, and its rows after the header.
func csvShape(path string) (shape, error) {
	f, err := os.Open(path)
	if err != nil {
		return shape{}, err
	}
	defer f.Close()
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return shape{}, fmt.Errorf("reading the CSV header: %w", err)
	}
	s := shape{Cols: len(header)}
	for {
		if _, err := r.Read(); err == io.EOF {
			return s, nil
		} else if err != nil {
			return s, fmt.Errorf("reading CSV row %d: %w", s.Rows+1, err)
		}
		s.Rows++
	}
}

const pgDSN = "postgres://bench:bench@127.0.0.1:55432/bench?sslmode=disable"

func psqlTool() tool {
	return cliTool{
		name: "psql",
		env:  []string{"PGPASSWORD=bench"},
		argv: func(sql, out string) []string {
			return []string{"psql", "-X", "-q", "--csv", "-h", "127.0.0.1", "-p", "55432", "-U", "bench", "-d", "bench", "-o", out, "-c", sql}
		},
	}
}

func usqlTool(bin string) tool {
	return cliTool{
		name: "usql",
		argv: func(sql, out string) []string {
			return []string{bin, "-X", "-q", "--csv", "-o", out, "-c", sql, pgDSN}
		},
	}
}

// anfraCLITool is `anfra query export`, in repoDir: cold when no server runs
// for that repo, warm (forwarded to it) when one does.
func anfraCLITool(name, repoDir string) tool {
	return cliTool{
		name: name,
		dir:  repoDir,
		argv: func(sql, out string) []string {
			return []string{"anfra", "--non-interactive", "query", "export", "--lang", "sql", "-s", "bench_pg",
				"--cache.duration", "0s", "--format-options.header", "names", "-o", out, sql}
		},
	}
}

// --- HTTP tools: time from request to the whole answer read and parsed. ----------

var httpClient = &http.Client{Timeout: 10 * time.Minute}

// anfraHTTPTool is core.query on anfra serve, streamed as NDJSON, as the SDK
// (and so a Data App) asks for it.
type anfraHTTPTool struct{ base string }

func (anfraHTTPTool) Name() string { return "anfra-http" }
func (anfraHTTPTool) Path() string { return "http" }

func (t anfraHTTPTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	body, _ := json.Marshal(map[string]any{
		"query": c.SQL, "lang": "sql", "data_source": "bench_pg",
		"cache": map[string]any{"duration": "0s"},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/api/core.query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return time.Since(start), shape{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return time.Since(start), shape{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	var s shape
	var failure error
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	for sc.Scan() {
		var ev struct {
			Type    string          `json:"type"`
			Columns []any           `json:"columns"`
			Records [][]any         `json:"records"`
			Status  string          `json:"status"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return time.Since(start), s, fmt.Errorf("parsing an event: %w", err)
		}
		switch ev.Type {
		case "header":
			s.Cols = len(ev.Columns)
		case "rows":
			s.Rows += len(ev.Records)
		case "end":
			if ev.Status != "ok" && ev.Status != "success" && len(ev.Error) > 0 {
				failure = fmt.Errorf("stream ended %s: %s", ev.Status, ev.Error)
			}
		}
	}
	took := time.Since(start)
	if err := sc.Err(); err != nil {
		return took, s, err
	}
	return took, s, failure
}

// supersetTool is SQL Lab's execute API, synchronous, as Superset's own SQL
// Lab page calls it.
type supersetTool struct {
	base       string
	databaseID int
	token      string
}

var supersetClientSeq atomic.Int64

// supersetClientID is a fresh id for a SQL Lab query: Superset keys queries by
// it, and takes at most 11 characters.
func supersetClientID() string {
	id := strconv.FormatInt(time.Now().UnixMilli()%1e9, 36) + strconv.FormatInt(supersetClientSeq.Add(1)%1e4, 36)
	return "b" + id
}

func newSupersetTool(ctx context.Context, base string) (*supersetTool, error) {
	body := `{"username":"admin","password":"admin","provider":"db","refresh":false}`
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/security/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("logging in to Superset: %w", err)
	}
	defer resp.Body.Close()
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil || login.AccessToken == "" {
		return nil, fmt.Errorf("logging in to Superset: HTTP %d, %v", resp.StatusCode, err)
	}
	return &supersetTool{base: base, databaseID: 1, token: login.AccessToken}, nil
}

func (*supersetTool) Name() string { return "superset" }
func (*supersetTool) Path() string { return "http" }

func (t *supersetTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	body, _ := json.Marshal(map[string]any{
		"database_id": t.databaseID, "sql": c.SQL, "runAsync": false,
		"queryLimit": 2_000_000, "client_id": supersetClientID(),
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/api/v1/sqllab/execute/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.token)

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return time.Since(start), shape{}, err
	}
	defer resp.Body.Close()
	var answer struct {
		Status  string           `json:"status"`
		Data    []map[string]any `json:"data"`
		Columns []any            `json:"columns"`
	}
	err = json.NewDecoder(resp.Body).Decode(&answer)
	took := time.Since(start)
	if err != nil {
		return took, shape{}, fmt.Errorf("HTTP %d, parsing the answer: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || answer.Status != "success" {
		return took, shape{}, fmt.Errorf("HTTP %d, status %q", resp.StatusCode, answer.Status)
	}
	return took, shape{Rows: len(answer.Data), Cols: len(answer.Columns)}, nil
}

// metabaseTool is Metabase's dataset API with a native query, as its question
// editor runs SQL, with a session from one login.
type metabaseTool struct {
	base       string
	databaseID int
	session    string
}

func newMetabaseTool(ctx context.Context, base string) (*metabaseTool, error) {
	body := `{"username":"admin@bench.invalid","password":"bench-Metabase-1"}`
	var login struct {
		ID string `json:"id"`
	}
	if err := callJSON(ctx, http.MethodPost, base+"/api/session", body, nil, &login); err != nil || login.ID == "" {
		return nil, fmt.Errorf("logging in to Metabase (run metabase/setup.sh first): %v", err)
	}
	header := map[string]string{"X-Metabase-Session": login.ID}
	var databases struct {
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := callJSON(ctx, http.MethodGet, base+"/api/database", "", header, &databases); err != nil {
		return nil, fmt.Errorf("listing Metabase's databases: %w", err)
	}
	for _, d := range databases.Data {
		if d.Name == "bench_pg" {
			return &metabaseTool{base: base, databaseID: d.ID, session: login.ID}, nil
		}
	}
	return nil, fmt.Errorf("Metabase has no database bench_pg: run metabase/setup.sh")
}

func (*metabaseTool) Name() string { return "metabase" }
func (*metabaseTool) Path() string { return "http" }

func (t *metabaseTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	body, _ := json.Marshal(map[string]any{
		"database": t.databaseID, "type": "native", "native": map[string]any{"query": c.SQL},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/api/dataset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Metabase-Session", t.session)

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return time.Since(start), shape{}, err
	}
	defer resp.Body.Close()
	var answer struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Rows [][]any `json:"rows"`
			Cols []any   `json:"cols"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&answer)
	took := time.Since(start)
	if err != nil {
		return took, shape{}, fmt.Errorf("HTTP %d, parsing the answer: %w", resp.StatusCode, err)
	}
	if answer.Status != "completed" {
		return took, shape{}, fmt.Errorf("HTTP %d, status %q: %s", resp.StatusCode, answer.Status, answer.Error)
	}
	return took, shape{Rows: len(answer.Data.Rows), Cols: len(answer.Data.Cols)}, nil
}

// callJSON sends a request with an optional JSON body and headers, and decodes
// a JSON answer into out.
func callJSON(ctx context.Context, method, url, body string, header map[string]string, out any) error {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
