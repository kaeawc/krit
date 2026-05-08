package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	mode := flag.String("mode", "findings", "stat to print: findings, source-findings, rules, files, sarif-results, oracle-bench-env, unix-ms")
	file := flag.String("file", "", "input file; stdin when empty")
	flag.Parse()

	var r io.Reader = os.Stdin
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	if *mode == "unix-ms" {
		fmt.Println(time.Now().UnixMilli())
		return
	}

	var data map[string]any
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch *mode {
	case "findings":
		fmt.Println(arrayLen(data["findings"]))
	case "source-findings":
		fmt.Println(sourceFindings(data))
	case "rules":
		fmt.Println(ruleCount(data))
	case "files":
		fmt.Println(numberValue(data["files"]))
	case "sarif-results":
		fmt.Println(sarifResults(data))
	case "oracle-bench-env":
		printOracleBenchEnv(data)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func findings(data map[string]any) []any {
	a, _ := data["findings"].([]any)
	return a
}

func sourceFindings(data map[string]any) int {
	total := 0
	for _, item := range findings(data) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		file, _ := m["file"].(string)
		if !contains(file, "/test/") {
			total++
		}
	}
	return total
}

func ruleCount(data map[string]any) int {
	seen := map[string]bool{}
	for _, item := range findings(data) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := m["rule"].(string)
		if rule != "" {
			seen[rule] = true
		}
	}
	return len(seen)
}

func arrayLen(v any) int {
	a, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(a)
}

func numberValue(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func sarifResults(data map[string]any) int {
	runs, ok := data["runs"].([]any)
	if !ok {
		return 0
	}
	total := 0
	for _, run := range runs {
		m, ok := run.(map[string]any)
		if !ok {
			continue
		}
		total += arrayLen(m["results"])
	}
	return total
}

func printOracleBenchEnv(data map[string]any) {
	timings, _ := data["perfTiming"].([]any)
	metrics := func(path ...string) map[string]any {
		if e := findTiming(timings, path...); e != nil {
			if m, ok := e["metrics"].(map[string]any); ok {
				return m
			}
		}
		return map[string]any{}
	}
	fmt.Printf("total_ms=%d\n", numberValue(data["durationMs"]))
	fmt.Printf("oracle_ms=%d\n", timingMS(timings, "typeOracle"))
	fmt.Printf("jvm_ms=%d\n", timingMS(timings, "typeOracle", "jvmAnalyze"))
	fmt.Printf("process_ms=%d\n", timingMS(timings, "typeOracle", "jvmAnalyze", "kritTypesProcess"))
	fmt.Printf("build_ms=%d\n", timingMS(timings, "typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinBuildSession"))
	fmt.Printf("analyze_ms=%d\n", timingMS(timings, "typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinAnalyzeFiles"))
	fmt.Printf("json_build_ms=%d\n", timingMS(timings, "typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinOracleJsonBuild"))
	fmt.Printf("parse_ms=%d\n", timingMS(timings, "parse"))
	fmt.Printf("type_idx_ms=%d\n", timingMS(timings, "typeIndex"))
	fmt.Printf("rule_exec_ms=%d\n", timingMS(timings, "ruleExecution"))
	fmt.Printf("findings=%d\n", arrayLen(data["findings"]))
	fmt.Printf("rules=%d\n", ruleCount(data))

	filter := metrics("typeOracle", "jvmAnalyze", "oracleFilterSummary")
	call := metrics("typeOracle", "jvmAnalyze", "oracleCallFilterSummary")
	analyze := metrics("typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinAnalyzeSummary")
	rss := metrics("typeOracle", "jvmAnalyze", "kritTypesProcessResources")
	fmt.Printf("filter_files=%v/%v\n", metric(filter, "markedFiles"), metric(filter, "totalFiles"))
	fmt.Printf("callee_names=%v\n", metric(call, "calleeNames"))
	fmt.Printf("lexical_hints=%v\n", metric(call, "lexicalHints"))
	fmt.Printf("lexical_skips=%v\n", metric(call, "lexicalSkips"))
	fmt.Printf("kt_files_analyzed=%v\n", metric(analyze, "files"))
	fmt.Printf("peak_rss_mb=%v\n", metric(rss, "peakRSSMB"))
}

func timingMS(entries []any, path ...string) int {
	if e := findTiming(entries, path...); e != nil {
		return numberValue(e["durationMs"])
	}
	return 0
}

func findTiming(entries []any, path ...string) map[string]any {
	if len(path) == 0 {
		return nil
	}
	for _, item := range entries {
		m, ok := item.(map[string]any)
		if !ok || m["name"] != path[0] {
			continue
		}
		if len(path) == 1 {
			return m
		}
		children, _ := m["children"].([]any)
		return findTiming(children, path[1:]...)
	}
	return nil
}

func metric(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	return "?"
}

func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
