// Package traces implements the `krit traces` CLI verb. Subcommands:
//
//	ingest     read OTel/JFR/JUnit input and merge into the store.
//	overlay    emit static graph + runtime overlay as JSON.
//	orphans    list static symbols never observed at runtime.
//	phantoms   list runtime states no static symbol resolves to.
//	divergence diff runtime evidence between two commits.
//
// The verb operates on a per-repo store at `.krit/traces/store.json`
// (sibling to `.krit/snapshots`). Static reconciliation reads the
// closest available snapshot's symbol list — capture one with
// `krit snapshot capture` before invoking overlay / orphans.
package traces

import (
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/cli/clishared"
)

const usage = `usage: krit traces <ingest|overlay|orphans|phantoms|divergence> [flags]

  ingest     read OTel/JFR/JUnit input and merge into the store
  overlay    emit static graph + runtime overlay as JSON
  orphans    list static symbols never observed at runtime
  phantoms   list runtime states no static symbol resolves to
  divergence diff runtime evidence between two commits

Ingest flags (exactly one of --otel, --jfr, --junit-steps is required):
  --otel PATH         OTLP/JSON span file
  --jfr PATH          jfr-print --json sample file
  --junit-steps PATH  JUnit step-boundary JSON file
  --commit SHA        record the commit this run is for (optional)
  --env ENV           record the environment label (optional)
  --repo PATH         repository root (default: cwd)

Overlay / orphans / phantoms flags:
  --repo PATH         repository root (default: cwd)
  --commit SHA        snapshot sha to reconcile against (default: latest captured)
  --json              emit machine-readable JSON (default: human summary)

Divergence flags:
  --from SHA          baseline snapshot sha (required)
  --to SHA            comparison snapshot sha (required)
  --repo PATH         repository root (default: cwd)
`

// Run is the entry point invoked by cmd/krit/verb_dispatch.go.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}
	switch args[0] {
	case "ingest":
		return runIngest(args[1:])
	case "overlay":
		return runOverlay(args[1:])
	case "orphans":
		return runOrphans(args[1:])
	case "phantoms":
		return runPhantoms(args[1:])
	case "divergence":
		return runDivergence(args[1:])
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown traces subcommand: %s\n%s", args[0], usage)
		return 1
	}
}

// resolveRepoRoot delegates to clishared so the verb honors the
// same --repo convention as `krit snapshot`, `krit bisect`, etc.
func resolveRepoRoot(flagValue string) (string, int) {
	return clishared.ResolveRepoRoot(flagValue)
}
