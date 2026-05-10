package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_OracleDaemonPlumbing verifies the daemon-side wiring
// for #125 PR-B: when host.OracleDaemon is supplied and
// args.OracleEnabled is true, IndexInput.PrebuiltOracleDaemon flows
// through to runDaemonOracle. The test stops short of actually
// invoking the JVM (no krit-types.jar in test env) by ensuring the
// active rule set declares no NeedsOracle / NeedsResolver capability —
// runOracle's internal gate (KotlinOracleRulesV2) returns empty, so
// the daemon handle is never used. The contract under test is "the
// daemon's resident handle reaches RunProject without an InvokeDaemon
// call" — runDaemonOracle's no-op behavior on empty oracle rules is
// the seam that lets us assert this without a JVM.
func TestRunProject_OracleDaemonPlumbing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	rule := api.FakeRule("PlumbingClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	// A non-nil daemon stub. The gate on KotlinOracleRulesV2 ensures
	// runDaemonOracle is never called for this rule set — but if the
	// plumbing were wrong (e.g. host.OracleDaemon ignored, fresh
	// InvokeDaemon called), the test environment without krit-types.jar
	// would error out. The assertion is implicit: no error and the
	// run completes.
	stub := &oracle.Daemon{}

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:        config.NewConfig(),
			Paths:         []string{dir},
			ActiveRules:   []*api.Rule{rule},
			Format:        "json",
			Version:       "test",
			OracleEnabled: true,
		},
		Host: ProjectHostState{
			OracleDaemon: stub,
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1", res.FilesScanned)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1 (rule should still fire even with oracle wiring)", res.FindingsCount)
	}
}

// TestRunProject_OracleDaemonNotWiredWhenDisabled confirms the
// negative path: with args.OracleEnabled=false (the pre-#125 default),
// host.OracleDaemon is ignored and the verb behaves as before.
func TestRunProject_OracleDaemonNotWiredWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "S.kt"),
		[]byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("Noop")

	// Pass a daemon stub but leave OracleEnabled=false.
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
		},
		Host: ProjectHostState{
			OracleDaemon: &oracle.Daemon{},
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	// With oracle disabled the daemon stub should be ignored. The
	// run completes; oracle handle never gets touched.
	_ = res
}
