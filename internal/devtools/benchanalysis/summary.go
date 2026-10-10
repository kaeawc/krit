//go:build unix

package main

import (
	"fmt"
	"sort"
	"strings"
)

// Stats summarizes accepted measured runs for one mode and state.
type Stats struct {
	N                      int
	Median, P90, Min, Max  int64
	MedianCPU, MedianRSSKB int64
}

func stats(runs []Run) Stats {
	var wall, cpu, rss []int64
	for _, r := range runs {
		wall = append(wall, r.WallMs)
		cpu = append(cpu, r.UserCPUMs+r.SysCPUMs)
		rss = append(rss, r.MaxRSSKB)
	}
	if len(wall) == 0 {
		return Stats{}
	}
	sorted := func(v []int64) []int64 {
		out := append([]int64(nil), v...)
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	w := sorted(wall)
	return Stats{
		N: len(w), Median: percentile(w, 50), P90: percentile(w, 90), Min: w[0], Max: w[len(w)-1],
		MedianCPU: percentile(sorted(cpu), 50), MedianRSSKB: percentile(sorted(rss), 50),
	}
}

// percentile uses nearest-rank on a sorted slice.
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p*len(sorted) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	return sorted[rank-1]
}

func summarize(results []Run, o options) string {
	var b strings.Builder
	b.WriteString("| mode | state | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | verified | findings |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	var rejected []string
	for _, m := range o.modes {
		for _, s := range o.states {
			var measured, accepted []Run
			verified, checked := 0, 0
			findings := map[int]bool{}
			for _, r := range results {
				if r.Mode == m && r.State == s && !r.Measured && r.Rejected != "" {
					rejected = append(rejected, fmt.Sprintf("- %s/%s fill run (state skipped): %s", m, s, r.Rejected))
				}
				if r.Mode != m || r.State != s || !r.Measured {
					continue
				}
				measured = append(measured, r)
				if r.Verified != nil {
					checked++
					if *r.Verified {
						verified++
					}
				}
				if r.Rejected != "" {
					rejected = append(rejected, fmt.Sprintf("- %s/%s run %d: %s", m, s, r.Index, r.Rejected))
					continue
				}
				accepted = append(accepted, r)
				findings[r.Findings] = true
			}
			st := stats(accepted)
			ver := "n/a"
			if checked > 0 {
				ver = fmt.Sprintf("%d/%d", verified, checked)
			}
			fmt.Fprintf(&b, "| %s | %s | %d/%d | %d | %d | %d–%d | %d | %d | %s | %s |\n",
				m, s, st.N, len(measured), st.Median, st.P90, st.Min, st.Max, st.MedianCPU, st.MedianRSSKB/1024, ver, findingCounts(findings))
		}
	}
	if len(rejected) > 0 {
		b.WriteString("\nRejected runs (excluded from timings):\n")
		b.WriteString(strings.Join(rejected, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\nCPU and RSS cover the krit process and the helpers it waited for; long-lived daemons are not included. Raw data: results.json.\n")
	return b.String()
}

func findingCounts(set map[int]bool) string {
	var counts []int
	for n := range set {
		counts = append(counts, n)
	}
	sort.Ints(counts)
	parts := make([]string, len(counts))
	for i, n := range counts {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ",")
}
