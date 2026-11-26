package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// composeHeavyFile writes and parses a Compose file with n composables, each making several calls, as in #1664.
func composeHeavyFile(b *testing.B, n int) *scanner.File {
	b.Helper()
	var sb strings.Builder
	sb.WriteString("package bench\n\nimport androidx.compose.runtime.*\nimport androidx.compose.foundation.layout.Column\n" +
		"import androidx.compose.material3.Text\nimport androidx.compose.foundation.interaction.MutableInteractionSource\n\n")
	for i := range n {
		fmt.Fprintf(&sb, "class Model%d(val name: String)\n\n@Composable\nfun Screen%d(model: Model%d) {\n", i, i, i)
		sb.WriteString("    val count = remember { mutableStateOf(0) }\n    val source = remember { MutableInteractionSource() }\n")
		sb.WriteString("    Column {\n        Text(model.name)\n        Text(\"${count.value}\")\n")
		sb.WriteString("        listOf(1, 2, 3).map { it * 2 }.filter { it > 2 }.forEach { println(it) }\n    }\n}\n\n")
	}
	path := filepath.Join(b.TempDir(), "Screens.kt")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatal(err)
	}
	return f
}

// BenchmarkComposeHeavyFileRules times the rules #1664 found dominating rule time on a 300-composable file. The
// dispatcher is rebuilt per iteration so its per-run file-facts cache starts cold, as in a scan.
func BenchmarkComposeHeavyFileRules(b *testing.B) {
	file := composeHeavyFile(b, 300)
	for _, id := range []string{"UnrememberedMutableState", "RememberInComposition", "IgnoredReturnValue"} {
		var rule *api.Rule
		for _, r := range api.Registry {
			if r.ID == id {
				rule = r
			}
		}
		if rule == nil {
			b.Fatalf("rule %s not registered", id)
		}
		b.Run(id, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = rules.NewDispatcher([]*api.Rule{rule}, nil).Run(file)
			}
		})
	}
}
