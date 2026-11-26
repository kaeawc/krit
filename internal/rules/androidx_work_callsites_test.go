package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func workFindings(t *testing.T, ruleID, name, src string, profile *librarymodel.ProjectProfile) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var file *scanner.File
	var err error
	if filepath.Ext(name) == ".java" {
		file, err = scanner.ParseJavaFile(context.Background(), path)
	} else {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range api.Registry {
		if r.ID != ruleID {
			continue
		}
		d := NewDispatcher([]*api.Rule{r}, nil)
		if profile != nil {
			d.SetLibraryFacts(librarymodel.FactsForProfile(*profile))
		}
		cols := d.Run(file)
		return cols.Findings()
	}
	t.Fatalf("rule %s not registered", ruleID)
	return nil
}

const workIdleSource = `package com.example

import androidx.work.Constraints

fun build() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
}
`

func TestAndroidxWorkChecksSkipProjectsProvenWithoutWorkManager(t *testing.T) {
	without := &librarymodel.ProjectProfile{HasGradle: true, DependencyExtractionComplete: true,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.core", Name: "core-ktx", Version: "1.15.0"}}}
	if got := workFindings(t, "IdleBatteryChargingConstraints", "A.kt", workIdleSource, without); len(got) != 0 {
		t.Fatalf("a project whose complete dependency list lacks androidx.work has no work lint.jar; got %d findings", len(got))
	}
	with := &librarymodel.ProjectProfile{HasGradle: true, DependencyExtractionComplete: true,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.work", Name: "work-runtime-ktx", Version: "2.12.0"}}}
	if got := workFindings(t, "IdleBatteryChargingConstraints", "A.kt", workIdleSource, with); len(got) != 1 {
		t.Fatalf("with work-runtime-ktx: got %d findings, want 1", len(got))
	}
	partial := &librarymodel.ProjectProfile{HasGradle: true}
	if got := workFindings(t, "IdleBatteryChargingConstraints", "A.kt", workIdleSource, partial); len(got) != 1 {
		t.Fatalf("an incomplete dependency list must not hide findings: got %d, want 1", len(got))
	}
}

func TestAndroidxWorkChecksSkipTestSources(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "src", "test", "java")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(path, []byte(workIdleSource), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range api.Registry {
		if r.ID == "IdleBatteryChargingConstraints" {
			if cols := NewDispatcher([]*api.Rule{r}, nil).Run(file); cols.Len() != 0 {
				t.Fatalf("Lint skips test sources by default; got %d findings", cols.Len())
			}
		}
	}
}

func TestWorkRootIdentifierMatchesLintsIdentifierName(t *testing.T) {
	src := `package com.example

fun f() {
    a.b.setX(true)
    a.setX(true).setY(true)
    Builder().setX(true)
    this.setX(true)
    (a).setX(true)
}
`
	path := filepath.Join(t.TempDir(), "A.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		name  string
		named bool
	}
	var roots []got
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		call, ok := lintCallAt(file, idx)
		if !ok || call.Name != "setX" {
			return
		}
		name, named := workRootIdentifier(file, call.ReceiverNode)
		roots = append(roots, got{name, named})
	})
	want := []got{{"a", true}, {"a", true}, {"", false}, {"", false}, {"a", true}}
	if len(roots) != len(want) {
		t.Fatalf("roots = %v, want %v", roots, want)
	}
	for i := range want {
		if roots[i] != want[i] {
			t.Fatalf("roots = %v, want %v", roots, want)
		}
	}
}

func TestInvalidPeriodicWorkRequestIntervalJavaNeedsLongArguments(t *testing.T) {
	src := `package com.example;

import androidx.work.ListenableWorker;
import androidx.work.PeriodicWorkRequest;
import java.time.Duration;
import java.util.concurrent.TimeUnit;

class J {
    static final int FIVE = 5;
    static final long SIX = 6L;

    void f(Class<? extends ListenableWorker> w) {
        new PeriodicWorkRequest.Builder(w, 10, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(w, FIVE, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(w, Duration.ofMinutes(5));
        new PeriodicWorkRequest.Builder(w, SIX, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(w, Duration.ofMinutes(5L));
    }
}
`
	got := workFindings(t, "InvalidPeriodicWorkRequestInterval", "J.java", src, nil)
	if len(got) != 2 || got[0].Line != 16 || got[1].Line != 17 {
		t.Fatalf("want findings on lines 16 and 17 only (the detector skips int-typed values), got %+v", got)
	}
}
