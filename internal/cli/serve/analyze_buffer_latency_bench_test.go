package serve

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// BenchmarkAnalyzeBufferEdit measures the per-file path the IntelliJ plugin
// takes after each edit (issue #538 targets under 200 ms): one
// analyze-buffer round trip on a warm daemon. The content changes on every
// call, so the daemon's buffer cache never answers. Reports the p50, p95,
// and max round trip for each file size.
//
//	scripts/go-test.sh ./internal/cli/serve/ -run '^$' -bench BenchmarkAnalyzeBufferEdit -benchtime 200x -count=1
func BenchmarkAnalyzeBufferEdit(b *testing.B) {
	for _, functions := range []int{10, 50, 100} {
		content := benchKotlinFile(functions)
		lines := strings.Count(content, "\n")
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			socket, _ := startServerWith(b, b.TempDir(), 0)
			call := func(i int) {
				args := daemon.AnalyzeBufferArgs{
					Path:    "src/main/kotlin/com/example/Bench.kt",
					Content: content + fmt.Sprintf("// edit %d\n", i),
				}
				var got daemon.AnalyzeBufferResult
				if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer, args, &got); err != nil {
					b.Fatalf("analyze-buffer: %v", err)
				}
				if got.CacheHit {
					b.Fatalf("call %d hit the buffer cache; the benchmark must measure a fresh scan", i)
				}
			}
			// Warm the daemon (rule registry, config, parser pools).
			for i := 0; i < 5; i++ {
				call(-1 - i)
			}
			durations := make([]time.Duration, 0, b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				call(i)
				durations = append(durations, time.Since(start))
			}
			b.StopTimer()
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
			b.ReportMetric(ms(durations[(len(durations)-1)*50/100]), "p50_ms")
			b.ReportMetric(ms(durations[(len(durations)-1)*95/100]), "p95_ms")
			b.ReportMetric(ms(durations[len(durations)-1]), "max_ms")
		})
	}
}

// benchKotlinFile returns a plausible application source file: a class with
// properties, null handling, collection pipelines, string templates, and a
// when expression in each of n functions (about 10 lines per function).
func benchKotlinFile(n int) string {
	var sb strings.Builder
	sb.WriteString("package com.example\n\n")
	sb.WriteString("import java.io.File\n\n")
	sb.WriteString("class Bench(private val root: File, private val names: List<String>) {\n")
	sb.WriteString("    private var cache: MutableMap<String, Int> = mutableMapOf()\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "    fun step%d(input: String?): Int {\n", i)
		sb.WriteString("        val value = input ?: return -1\n")
		sb.WriteString("        val counts = names.filter { it.startsWith(value) }.map { it.length }\n")
		fmt.Fprintf(&sb, "        val label = \"step%d: ${counts.sum()} in ${root.name}\"\n", i)
		sb.WriteString("        return when {\n")
		sb.WriteString("            counts.isEmpty() -> cache.getOrPut(label) { 0 }\n")
		sb.WriteString("            counts.size > 3 -> counts.maxOrNull() ?: 0\n")
		sb.WriteString("            else -> label.length\n")
		sb.WriteString("        }\n")
		sb.WriteString("    }\n\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}
