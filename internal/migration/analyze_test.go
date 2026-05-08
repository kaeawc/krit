package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeReportsMigrationSuggestion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app", "src", "main", "kotlin", "Api.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`
package com.example

import retrofit2.adapter.rxjava2.RxJava2CallAdapterFactory

fun createAdapter() {
    Retrofit.Builder().addCallAdapterFactory(RxJava2CallAdapterFactory.create())
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Analyze(Options{
		Root:    root,
		Library: "retrofit",
		From:    "2.9.0",
		To:      "2.10.0",
		Map: Map{
			Library: "retrofit",
			Migrations: []Migration{{
				From: "2.9.0",
				To:   "2.10.0",
				Symbols: []Entry{{
					Symbol:      "retrofit2.adapter.rxjava2.RxJava2CallAdapterFactory.create",
					Replacement: "retrofit2.adapter.rxjava3.RxJava3CallAdapterFactory.create",
					Reason:      "use RxJava3 adapter",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Suggestions) != 2 {
		t.Fatalf("suggestions = %+v, want import and call suggestions", report.Suggestions)
	}
	if got := report.Suggestions[0].Suggested; got != "import retrofit2.adapter.rxjava3.RxJava3CallAdapterFactory" {
		t.Fatalf("import suggested = %q", got)
	}
	call := report.Suggestions[1]
	if call.File != "app/src/main/kotlin/Api.kt" || call.Line == 0 {
		t.Fatalf("call suggestion = %+v", call)
	}
	if !strings.Contains(call.Suggested, "RxJava3CallAdapterFactory.create()") {
		t.Fatalf("suggested = %q", call.Suggested)
	}
}
