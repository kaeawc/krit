package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestIndexPhase_Name(t *testing.T) {
	if (IndexPhase{}).Name() != "index" {
		t.Fatalf("Name = %q, want index", (IndexPhase{}).Name())
	}
}

func TestIndexPhase_Run_NoResolver_WhenNoRuleNeedsIt(t *testing.T) {
	in := IndexInput{
		ParseResult: ParseResult{
			Paths: []string{t.TempDir()},
			ActiveRules: []*v2.Rule{
				{ID: "R", Description: "d", NodeTypes: nil, Check: func(*v2.Context) {}},
			},
		},
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver != nil {
		t.Errorf("Resolver = %v, want nil (no rule needs it)", out.Resolver)
	}
}

func TestIndexPhase_Run_BuildsResolver_WhenRuleNeedsIt(t *testing.T) {
	dir := t.TempDir()
	kt := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(kt, []byte("class A { val x: Int = 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(kt)
	if err != nil {
		t.Fatal(err)
	}

	in := IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{file},
			ActiveRules: []*v2.Rule{
				{ID: "T", Description: "d", Needs: v2.NeedsResolver, Check: func(*v2.Context) {}},
			},
		},
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver == nil {
		t.Fatal("Resolver is nil; expected a resolver since a rule declares NeedsResolver")
	}
}

func TestIndexPhase_Run_UsesPrebuiltResolver(t *testing.T) {
	prebuilt := typeinfer.NewResolver()
	in := IndexInput{
		ParseResult: ParseResult{
			Paths: []string{t.TempDir()},
			ActiveRules: []*v2.Rule{
				{ID: "T", Description: "d", Needs: v2.NeedsResolver, Check: func(*v2.Context) {}},
			},
		},
		PrebuiltResolver: prebuilt,
	}
	out, err := IndexPhase{SkipAndroid: true, SkipModules: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Resolver == nil {
		t.Fatal("Resolver unexpectedly nil despite PrebuiltResolver being non-nil")
	}
	// Identity — IndexPhase must pass through the pre-built resolver
	// unchanged, not construct a fresh one.
	if interface{}(out.Resolver) != interface{}(prebuilt) {
		t.Errorf("Resolver identity mismatch: got %v, want %v", out.Resolver, prebuilt)
	}
}

func TestIndexPhase_Run_ModuleGraphSkipped(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	out, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.ModuleGraph != nil {
		t.Errorf("ModuleGraph = %v, want nil when SkipModules=true", out.ModuleGraph)
	}
}

func TestIndexPhase_Run_AndroidSkipped(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	out, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.AndroidProject != nil {
		t.Errorf("AndroidProject = %v, want nil when SkipAndroid=true", out.AndroidProject)
	}
}

func TestIndexPhase_Run_ContextCancel(t *testing.T) {
	in := IndexInput{ParseResult: ParseResult{Paths: []string{t.TempDir()}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runPhase[IndexInput, IndexResult](ctx, IndexPhase{SkipModules: true, SkipAndroid: true}, in)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var pe *PhaseError
	if !errors.As(err, &pe) || pe.Phase != "index" {
		t.Fatalf("want PhaseError phase=index, got %v", err)
	}
}

