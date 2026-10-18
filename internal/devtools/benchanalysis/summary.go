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
	// Daemon columns cover long-lived daemons only; DaemonN is how many
	// accepted runs had a sample.
	DaemonN                         int
	MedianDaemonCPU, MedianDaemonKB int64
	Skips                           map[string]int
}

func stats(runs []Run) Stats {
	var wall, cpu, rss, dcpu, drss []int64
	skips := map[string]int{}
	for _, r := range runs {
		if r.OracleSkip != "" {
			key := r.OracleSkip
			if r.Verified == nil {
				key += ", unverified"
			}
			skips[key]++
		}
		if r.Daemon != nil && len(r.Daemon.PIDs) > 0 {
			dcpu = append(dcpu, r.Daemon.CPUMs)
			drss = append(drss, r.Daemon.PeakRSSKB)
		}
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
		DaemonN: len(dcpu), MedianDaemonCPU: percentile(sorted(dcpu), 50), MedianDaemonKB: percentile(sorted(drss), 50),
		Skips: skips,
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
	b.WriteString("| mode | run kind | accepted/measured | median ms | p90 ms | min–max ms | median CPU ms | median max RSS MB | daemon CPU ms | daemon lifetime-peak RSS MB | oracle | verified | findings |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
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
			dcpu, drss := "n/a", "n/a"
			if st.DaemonN > 0 {
				dcpu, drss = fmt.Sprint(st.MedianDaemonCPU), fmt.Sprint(st.MedianDaemonKB/1024)
			}
			fmt.Fprintf(&b, "| %s | %s | %d/%d | %d | %d | %d–%d | %d | %d | %s | %s | %s | %s | %s |\n",
				m, s, st.N, len(measured), st.Median, st.P90, st.Min, st.Max, st.MedianCPU, st.MedianRSSKB/1024, dcpu, drss, oracleColumn(m, st), ver, findingCounts(findings))
		}
	}
	if len(rejected) > 0 {
		b.WriteString("\nRejected runs (excluded from timings):\n")
		b.WriteString(strings.Join(rejected, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\nRun kinds: cold = every cache and process reset before each run; warm-disk = on-disk caches kept, fresh processes; warm-daemon = caches and krit and JVM daemons kept; body-edit and abi-edit = warm-daemon after a comment-only or public-API edit.\n")
	b.WriteString("CPU and max RSS cover the krit process and the helpers it waited for. Daemon CPU ms and daemon lifetime-peak RSS MB cover long-lived daemons (krit serve, krit-fir, krit-types, checker) sampled from /proc before and after each run; peak RSS is the daemons' lifetime VmHWM, and n/a means no sample (non-Linux). The oracle column says whether the oracle ran or why a warm run legitimately skipped it. Raw data: results.json.\n")
	return b.String()
}

// oracleColumn says whether accepted runs ran the oracle or which cache
// layer let them skip it.
func oracleColumn(m string, st Stats) string {
	if modes[m].Backend == "" {
		return "n/a"
	}
	if st.N == 0 {
		return "-"
	}
	if len(st.Skips) == 0 {
		return "ran"
	}
	var parts []string
	for k, n := range st.Skips {
		parts = append(parts, fmt.Sprintf("skipped: %s (%d)", k, n))
	}
	sort.Strings(parts)
	if ran := st.N - sumInts(st.Skips); ran > 0 {
		parts = append([]string{fmt.Sprintf("ran (%d)", ran)}, parts...)
	}
	return strings.Join(parts, "; ")
}

func sumInts(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
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
