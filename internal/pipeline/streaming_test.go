package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProjectStreaming_MatchesRunProject is the equivalence
// contract behind issue #60: the streaming entry point must produce
// byte-identical output to the buffered entry point so the daemon
// can swap between them without observable change. Stripping
// timing fields lets the comparison ignore the legitimately
// nondeterministic durationMs/wallSeconds keys.
func TestRunProjectStreaming_MatchesRunProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	mkInput := func() ProjectInput {
		return ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
		}
	}

	buffered, err := RunProject(context.Background(), mkInput())
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}

	var streamed bytes.Buffer
	res, err := RunProjectStreaming(context.Background(), mkInput(), &streamed)
	if err != nil {
		t.Fatalf("RunProjectStreaming: %v", err)
	}
	if res.JSON != nil {
		t.Errorf("RunProjectStreaming.JSON should be nil; got %d bytes", len(res.JSON))
	}
	if res.FindingsCount != buffered.FindingsCount {
		t.Errorf("FindingsCount mismatch: buffered=%d streamed=%d",
			buffered.FindingsCount, res.FindingsCount)
	}

	scrub := []string{
		"\"durationMs\"", "\"timestamp\"", "\"wallSeconds\"",
		"\"startTime\"", "\"endTime\"",
	}
	bufferedClean := scrubLines(buffered.JSON, scrub)
	streamedClean := scrubLines(streamed.Bytes(), scrub)
	if !bytes.Equal(bufferedClean, streamedClean) {
		t.Errorf("streamed output diverges from buffered\n--- buffered ---\n%s\n--- streamed ---\n%s",
			bufferedClean, streamedClean)
	}
}

// TestRunProjectStreaming_NilWriterRejected guards the nil-writer
// precondition. RunProjectStreaming with no destination is always a
// caller bug — failing fast beats letting the run silently succeed
// while writing nothing.
func TestRunProjectStreaming_NilWriterRejected(t *testing.T) {
	_, err := RunProjectStreaming(context.Background(), ProjectInput{}, nil)
	if err == nil {
		t.Fatal("expected error from nil writer")
	}
}

// TestRunProjectStreaming_JSONCompactSuppressesIndent confirms the
// JSONCompact knob in ProjectArgs is plumbed through to the JSON
// formatter so the daemon's wire-level body has no internal
// newlines (line-delimited daemon protocol breaks on the first '\n').
func TestRunProjectStreaming_JSONCompactSuppressesIndent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)
	in := ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
			JSONCompact: true,
		},
	}
	var buf bytes.Buffer
	if _, err := RunProjectStreaming(context.Background(), in, &buf); err != nil {
		t.Fatalf("RunProjectStreaming: %v", err)
	}
	body := buf.Bytes()
	// Encoder appends one trailing newline. Internal newlines would
	// indicate indentation slipped through.
	if n := bytes.Count(body, []byte{'\n'}); n != 1 {
		t.Errorf("compact JSON should have exactly one trailing newline; got %d in %q", n, body)
	}
}
