// Command runner measures how long each tool takes to run a query and deliver
// its whole result, over its path: to a file from a CLI, or to an HTTP client.
//
// It interleaves the tools: each round runs every (tool, case) once, in a
// shuffled order, so drift during the run spreads across all of them. Each
// measured run is one JSON line in -out; `runner report` summarizes them.
//
// Usage, from the query folder:
//
//	go -C runner build -o ../.bin/runner . && .bin/runner -out results/<machine>/batch1.jsonl
//	.bin/runner report results/<machine>/batch1.jsonl
//	.bin/runner cases > cases/cases.jsonl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// run is one measured run of a case by a tool, as written to the results.
type run struct {
	Batch int     `json:"batch"`
	DB    string  `json:"db"`
	Tool  string  `json:"tool"`
	Path  string  `json:"path"`
	Case  string  `json:"case"`
	Round int     `json:"round"`
	Ms    float64 `json:"ms"`
	shape
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "cases" {
		if err := writeCases(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "report" {
		if err := report(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := measure(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func measure(ctx context.Context) error {
	var (
		benchDir = flag.String("bench", ".", "the query benchmark folder")
		out      = flag.String("out", "", "the JSON Lines file to append runs to")
		batch    = flag.Int("batch", 1, "the batch number recorded with each run")
		rounds   = flag.Int("rounds", 5, "measured rounds")
		warmup   = flag.Int("warmup", 1, "unrecorded rounds before them")
		toolList = flag.String("tools", "psql,usql,anfra-cli-cold,anfra-cli-warm,anfra-http,superset,metabase", "the tools to run")
		caseList = flag.String("cases", "", "the cases to run, all by default")
		skipList = flag.String("skip", "", "tool:case cells not to run")
		addr     = flag.String("anfra-addr", "127.0.0.1:58090", "where to run anfra serve")
		superset = flag.String("superset", "http://127.0.0.1:58088", "Superset's base URL")
		metabase = flag.String("metabase", "http://127.0.0.1:58089", "Metabase's base URL")
	)
	flag.Parse()
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	bench, err := filepath.Abs(*benchDir)
	if err != nil {
		return err
	}

	cases := allCases
	if *caseList != "" {
		names := strings.Split(*caseList, ",")
		cases = slices.DeleteFunc(slices.Clone(allCases), func(c benchCase) bool { return !slices.Contains(names, c.Name) })
	}
	skip := map[string]bool{}
	for _, cell := range strings.Split(*skipList, ",") {
		skip[cell] = cell != ""
	}

	warmRepo := filepath.Join(bench, "anfra-repo")
	tools := map[string]func() (tool, error){
		"psql":           func() (tool, error) { return psqlTool(), nil },
		"usql":           func() (tool, error) { return usqlTool(filepath.Join(bench, ".bin", "usql")), nil },
		"anfra-cli-cold": func() (tool, error) { d, err := coldRepo(warmRepo); return anfraCLITool("anfra-cli-cold", d), err },
		"anfra-cli-warm": func() (tool, error) { return anfraCLITool("anfra-cli-warm", warmRepo), nil },
		"anfra-http":     func() (tool, error) { return anfraHTTPTool{base: "http://" + *addr}, nil },
		"superset":       func() (tool, error) { return newSupersetTool(ctx, *superset) },
		"metabase":       func() (tool, error) { return newMetabaseTool(ctx, *metabase) },
	}
	var chosen []tool
	needServer := false
	for _, name := range strings.Split(*toolList, ",") {
		mk, ok := tools[name]
		if !ok {
			return fmt.Errorf("no tool %q", name)
		}
		t, err := mk()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		chosen = append(chosen, t)
		needServer = needServer || name == "anfra-cli-warm" || name == "anfra-http"
	}
	if needServer {
		stopServer, err := startAnfraServe(ctx, warmRepo, *addr, filepath.Join(filepath.Dir(*out), "anfra-serve.log"))
		if err != nil {
			return err
		}
		defer stopServer()
	}

	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)

	type cell struct {
		t tool
		c benchCase
	}
	var cells []cell
	for _, t := range chosen {
		for _, c := range cases {
			if !skip[t.Name()+":"+c.Name] {
				cells = append(cells, cell{t, c})
			}
		}
	}
	for round := 1 - *warmup; round <= *rounds; round++ {
		rand.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
		roundStart := time.Now()
		failed := 0
		for _, x := range cells {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			took, s, err := x.t.Run(ctx, x.c)
			r := run{Batch: *batch, DB: "postgres", Tool: x.t.Name(), Path: x.t.Path(), Case: x.c.Name,
				Round: round, Ms: float64(took.Microseconds()) / 1000, shape: s}
			switch {
			case err != nil:
				r.Error = truncate(err.Error(), 500)
			case s.Rows != x.c.Rows || s.Cols != x.c.Cols:
				r.Error = fmt.Sprintf("returned %d rows × %d columns, want %d × %d", s.Rows, s.Cols, x.c.Rows, x.c.Cols)
			default:
				r.OK = true
			}
			if !r.OK {
				failed++
				fmt.Fprintf(os.Stderr, "  %s %s: %s\n", r.Tool, r.Case, r.Error)
			}
			if round >= 1 {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
		}
		label := fmt.Sprintf("round %d/%d", round, *rounds)
		if round < 1 {
			label = "warm-up"
		}
		fmt.Fprintf(os.Stderr, "%s: %d runs in %s, %d failed\n", label, len(cells), time.Since(roundStart).Round(time.Second), failed)
	}
	return nil
}

// coldRepo copies the bench repo's config to a fresh folder, a repo no server
// runs for, so `anfra` there runs each command itself, with its own sidecars.
func coldRepo(warmRepo string) (string, error) {
	dir, err := os.MkdirTemp("", "anfra-bench-cold-")
	if err != nil {
		return "", err
	}
	if err := os.CopyFS(dir, os.DirFS(warmRepo)); err != nil {
		return "", err
	}
	return dir, nil
}

// startAnfraServe runs anfra serve for repoDir, and waits until it is ready.
func startAnfraServe(ctx context.Context, repoDir, addr, logPath string) (stop func(), err error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("anfra", "--non-interactive", "serve", "--addr", addr, "--no-apps", "--no-mcp")
	cmd.Dir = repoDir
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop = func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
		}
		logFile.Close()
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if resp, err := http.Get("http://" + addr + "/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return stop, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	stop()
	return nil, fmt.Errorf("anfra serve did not become ready on %s; see %s", addr, logPath)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
