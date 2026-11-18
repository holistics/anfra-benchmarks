package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
)

// report prints a Markdown table of a results file: per case and tool, the
// median and slowest of its successful runs, with the failures counted.
func report(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: runner report <results.jsonl>")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	type key struct{ tool, kase string }
	times := map[key][]float64{}
	failures := map[key]int{}
	firstError := map[key]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r run
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return err
		}
		k := key{r.Tool, r.Case}
		if r.OK {
			times[k] = append(times[k], r.Ms)
		} else {
			failures[k]++
			if firstError[k] == "" {
				firstError[k] = r.Error
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	toolOrder := []string{"psql", "usql", "anfra-cli-cold", "anfra-cli-warm", "anfra-http", "superset", "metabase"}
	present := map[string]bool{}
	for k := range times {
		present[k.tool] = true
	}
	for k := range failures {
		present[k.tool] = true
	}
	var tools []string
	for _, t := range toolOrder {
		if present[t] {
			tools = append(tools, t)
		}
	}

	fmt.Printf("| case | %s |\n", strings.Join(tools, " | "))
	fmt.Printf("|---|%s\n", strings.Repeat("---:|", len(tools)))
	for _, c := range allCases {
		cells := make([]string, len(tools))
		any := false
		for i, t := range tools {
			k := key{t, c.Name}
			ms := times[k]
			switch {
			case len(ms) > 0:
				any = true
				cells[i] = fmt.Sprintf("%s (%s)", fmtMs(percentile(ms, 50)), fmtMs(slices.Max(ms)))
				if failures[k] > 0 {
					cells[i] += fmt.Sprintf(" ⚠%d failed", failures[k])
				}
			case failures[k] > 0:
				any = true
				cells[i] = "failed"
			default:
				cells[i] = "–"
			}
		}
		if any {
			fmt.Printf("| %s | %s |\n", c.Name, strings.Join(cells, " | "))
		}
	}

	var errs []string
	for k, e := range firstError {
		errs = append(errs, fmt.Sprintf("- %s · %s (%d failed): %s", k.tool, k.kase, failures[k], e))
	}
	sort.Strings(errs)
	if len(errs) > 0 {
		fmt.Println("\nFailures:")
		fmt.Println(strings.Join(errs, "\n"))
	}
	return nil
}

// percentile is the p-th percentile of ms, by nearest rank.
func percentile(ms []float64, p float64) float64 {
	s := slices.Clone(ms)
	slices.Sort(s)
	i := int(math.Ceil(p/100*float64(len(s)))) - 1
	return s[max(i, 0)]
}

func fmtMs(ms float64) string {
	switch {
	case ms >= 10_000:
		return fmt.Sprintf("%.1fs", ms/1000)
	case ms >= 100:
		return fmt.Sprintf("%.0f", ms)
	default:
		return fmt.Sprintf("%.1f", ms)
	}
}
