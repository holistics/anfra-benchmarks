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

// shape is a result's size, which must match its case's, and, for a semantic
// case, its values, which must match its answer's.
type shape struct {
	Rows   int        `json:"rows"`
	Cols   int        `json:"cols"`
	values [][]string // only for a semantic case
}

// --- CLI tools: time from start to exit, with the file fully written. -----------

// cliTool runs a command that writes the case's result to a CSV file with a
// header row.
type cliTool struct {
	name string
	dir  string
	env  []string
	argv func(c benchCase, out string) []string
}

func (t cliTool) Name() string { return t.name }
func (t cliTool) Path() string { return "file" }

func (t cliTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	out := filepath.Join(os.TempDir(), fmt.Sprintf("bench-%s-%d.csv", t.name, time.Now().UnixNano()))
	defer os.Remove(out)
	argv := t.argv(c, out)
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
	s, err := csvShape(out, c.Semantic != nil)
	return took, s, err
}

// csvShape counts a CSV file's columns, and its rows after the header; with
// keep, it keeps the rows' values too.
func csvShape(path string, keep bool) (shape, error) {
	f, err := os.Open(path)
	if err != nil {
		return shape{}, err
	}
	defer f.Close()
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.ReuseRecord = !keep
	header, err := r.Read()
	if err != nil {
		return shape{}, fmt.Errorf("reading the CSV header: %w", err)
	}
	s := shape{Cols: len(header)}
	for {
		record, err := r.Read()
		if err == io.EOF {
			return s, nil
		} else if err != nil {
			return s, fmt.Errorf("reading CSV row %d: %w", s.Rows+1, err)
		}
		s.Rows++
		if keep {
			s.values = append(s.values, record)
		}
	}
}

func psqlTool() tool {
	return cliTool{
		name: "psql",
		env:  []string{"PGPASSWORD=bench"},
		argv: func(c benchCase, out string) []string {
			sql := c.SQL
			return []string{"psql", "-X", "-q", "--csv", "-h", "127.0.0.1", "-p", "55432", "-U", "bench", "-d", "bench", "-o", out, "-c", sql}
		},
	}
}

// clickhouseClientTool is ClickHouse's own client, writing the result to a file
// itself (INTO OUTFILE), as CSV with a header row.
func clickhouseClientTool(bin string) tool {
	return cliTool{
		name: "clickhouse-client",
		argv: func(c benchCase, out string) []string {
			sql := c.SQL
			return []string{bin, "client", "--host", "127.0.0.1", "--port", "59000", "--user", "bench", "--password", "bench",
				"--database", "bench", "--query", fmt.Sprintf("%s INTO OUTFILE '%s' TRUNCATE FORMAT CSVWithNames", sql, out)}
		},
	}
}

// snowTool is the Snowflake CLI, writing the result to a file itself as CSV,
// on the connection snowflake/setup.py writes (.bin/snow/config.toml).
func snowTool(binDir string) tool {
	return cliTool{
		name: "snow",
		argv: func(c benchCase, out string) []string {
			sql := c.SQL
			return []string{filepath.Join(binDir, "snow-venv", "bin", "snow"), "--config-file", filepath.Join(binDir, "snow", "config.toml"),
				"sql", "-c", "bench", "-q", sql, "-o", "output_format=csv", "-o", "quiet=true", "-o", "output_file=" + out}
		},
	}
}

func usqlTool(bin, dsn string) tool {
	return cliTool{
		name: "usql",
		argv: func(c benchCase, out string) []string {
			return []string{bin, "-X", "-q", "--csv", "-o", out, "-c", c.SQL, dsn}
		},
	}
}

// anfraCLITool is `anfra query export`, in repoDir: cold when no server runs
// for that repo, warm (forwarded to it) when one does. A SQL case runs on the
// data source; a semantic case runs its AQL on the semantic dataset.
func anfraCLITool(name, repoDir, source string) tool {
	return cliTool{
		name: name,
		dir:  repoDir,
		argv: func(c benchCase, out string) []string {
			argv := []string{"anfra", "--non-interactive", "query", "export"}
			if c.Semantic != nil {
				argv = append(argv, "--dataset", semanticDataset)
			} else {
				argv = append(argv, "--lang", "sql", "-s", source)
			}
			query := c.SQL
			if c.Semantic != nil {
				query = c.Semantic.AQL
			}
			return append(argv, "--cache.duration", "0s", "--format-options.header", "names", "-o", out, query)
		},
	}
}

// --- HTTP tools: time from request to the whole answer read and parsed. ----------

var httpClient = &http.Client{Timeout: 10 * time.Minute}

// anfraHTTPTool is core.query on anfra serve, streamed as NDJSON, as the SDK
// (and so a Data App) asks for it.
type anfraHTTPTool struct{ base, source string }

func (anfraHTTPTool) Name() string { return "anfra-http" }
func (anfraHTTPTool) Path() string { return "http" }

func (t anfraHTTPTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	request := map[string]any{
		"query": c.SQL, "lang": "sql", "data_source": t.source,
		"cache": map[string]any{"duration": "0s"},
	}
	if c.Semantic != nil {
		request = map[string]any{"query": c.Semantic.AQL, "dataset": semanticDataset, "cache": map[string]any{"duration": "0s"}}
	}
	body, _ := json.Marshal(request)
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
			if c.Semantic != nil {
				for _, record := range ev.Records {
					s.values = append(s.values, textRow(record))
				}
			}
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
	loggedIn   time.Time
}

// supersetLogin is a fresh access token. Superset's expire after 15 minutes by
// default, so a long batch logs in again (see Run).
func supersetLogin(ctx context.Context, base string) (string, error) {
	var login struct {
		AccessToken string `json:"access_token"`
	}
	body := `{"username":"admin","password":"admin","provider":"db","refresh":false}`
	if err := callJSON(ctx, http.MethodPost, base+"/api/v1/security/login", body, nil, &login); err != nil || login.AccessToken == "" {
		return "", fmt.Errorf("logging in to Superset: %v", err)
	}
	return login.AccessToken, nil
}

var supersetClientSeq atomic.Int64

// supersetClientID is a fresh id for a SQL Lab query: Superset keys queries by
// it, and takes at most 11 characters.
func supersetClientID() string {
	id := strconv.FormatInt(time.Now().UnixMilli()%1e9, 36) + strconv.FormatInt(supersetClientSeq.Add(1)%1e4, 36)
	return "b" + id
}

func newSupersetTool(ctx context.Context, base, database string) (*supersetTool, error) {
	token, err := supersetLogin(ctx, base)
	if err != nil {
		return nil, err
	}
	var databases struct {
		Result []struct {
			ID   int    `json:"id"`
			Name string `json:"database_name"`
		} `json:"result"`
	}
	header := map[string]string{"Authorization": "Bearer " + token}
	if err := callJSON(ctx, http.MethodGet, base+"/api/v1/database/", "", header, &databases); err != nil {
		return nil, fmt.Errorf("listing Superset's databases: %w", err)
	}
	for _, d := range databases.Result {
		if d.Name == database {
			return &supersetTool{base: base, databaseID: d.ID, token: token, loggedIn: time.Now()}, nil
		}
	}
	return nil, fmt.Errorf("Superset has no database %s: see superset/start.sh", database)
}

func (*supersetTool) Name() string { return "superset" }
func (*supersetTool) Path() string { return "http" }

func (t *supersetTool) Run(ctx context.Context, c benchCase) (time.Duration, shape, error) {
	// A token near its expiry is renewed before the timer starts, so no run pays for a login.
	if time.Since(t.loggedIn) > 10*time.Minute {
		token, err := supersetLogin(ctx, t.base)
		if err != nil {
			return 0, shape{}, err
		}
		t.token, t.loggedIn = token, time.Now()
	}
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

func newMetabaseTool(ctx context.Context, base, database string) (*metabaseTool, error) {
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
		if d.Name == database {
			return &metabaseTool{base: base, databaseID: d.ID, session: login.ID}, nil
		}
	}
	return nil, fmt.Errorf("Metabase has no database %s: run metabase/setup.sh", database)
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
