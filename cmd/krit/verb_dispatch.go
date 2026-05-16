package main

import (
	"os"

	climabihash "github.com/kaeawc/krit/internal/cli/abihash"
	climapi "github.com/kaeawc/krit/internal/cli/api"
	climbisect "github.com/kaeawc/krit/internal/cli/bisect"
	climblastradius "github.com/kaeawc/krit/internal/cli/blastradius"
	climbreakage "github.com/kaeawc/krit/internal/cli/breakage"
	climcache "github.com/kaeawc/krit/internal/cli/cache"
	climdaemoncmd "github.com/kaeawc/krit/internal/cli/daemoncmd"
	climdeadcode "github.com/kaeawc/krit/internal/cli/deadcode"
	climdeltarisk "github.com/kaeawc/krit/internal/cli/deltarisk"
	climdigraph "github.com/kaeawc/krit/internal/cli/digraph"
	climeditorconfigdrift "github.com/kaeawc/krit/internal/cli/editorconfigdrift"
	climgen "github.com/kaeawc/krit/internal/cli/gen"
	climgraph "github.com/kaeawc/krit/internal/cli/graphexport"
	climharvest "github.com/kaeawc/krit/internal/cli/harvest"
	climimpact "github.com/kaeawc/krit/internal/cli/impact"
	climinit "github.com/kaeawc/krit/internal/cli/initcmd"
	climmetrics "github.com/kaeawc/krit/internal/cli/metrics"
	climmigrate "github.com/kaeawc/krit/internal/cli/migrate"
	climmocks "github.com/kaeawc/krit/internal/cli/mocks"
	climprecommit "github.com/kaeawc/krit/internal/cli/precommit"
	climrename "github.com/kaeawc/krit/internal/cli/rename"
	climriskmap "github.com/kaeawc/krit/internal/cli/riskmap"
	climrules "github.com/kaeawc/krit/internal/cli/rules"
	climscore "github.com/kaeawc/krit/internal/cli/score"
	climscorecard "github.com/kaeawc/krit/internal/cli/scorecard"
	climselecttests "github.com/kaeawc/krit/internal/cli/selecttests"
	climserve "github.com/kaeawc/krit/internal/cli/serve"
	climsnapshot "github.com/kaeawc/krit/internal/cli/snapshot"
	climsuggestreviewers "github.com/kaeawc/krit/internal/cli/suggestreviewers"
	climtestcoverage "github.com/kaeawc/krit/internal/cli/testcoverage"
	climtraces "github.com/kaeawc/krit/internal/cli/traces"
	climtransform "github.com/kaeawc/krit/internal/cli/transform"
	climtriage "github.com/kaeawc/krit/internal/cli/triage"
	climusedsymbols "github.com/kaeawc/krit/internal/cli/usedsymbols"
)

type subcommandVerb int

const (
	verbNone subcommandVerb = iota
	verbCache
	verbServe
	verbHarvest
	verbRename
	verbInit
	verbAPISnapshot
	verbAPIDiff
	verbABIHash
	verbImpact
	verbDeadCode
	verbDIGraph
	verbUsedSymbols
	verbTestCoverage
	verbSelectTests
	verbMocks
	verbTransform
	verbMigrate
	verbMetrics
	verbScore
	verbScorecard
	verbRiskMap
	verbBlastRadius
	verbBaselineAudit
	verbSuggestReviewers
	verbEditorConfigDrift
	verbGen
	verbGraph
	verbPrecommit
	verbSnapshot
	verbDaemon
	verbTriage
	verbRules
	verbBreakage
	verbBisectStructure
	verbDeltaRisk
	verbTraces
)

var verbByName = map[string]subcommandVerb{
	"cache":              verbCache,
	"serve":              verbServe,
	"harvest":            verbHarvest,
	"rename":             verbRename,
	"init":               verbInit,
	"api-snapshot":       verbAPISnapshot,
	"api-diff":           verbAPIDiff,
	"abi-hash":           verbABIHash,
	"impact":             verbImpact,
	"dead-code":          verbDeadCode,
	"di-graph":           verbDIGraph,
	"used-symbols":       verbUsedSymbols,
	"test-coverage":      verbTestCoverage,
	"select-tests":       verbSelectTests,
	"mocks":              verbMocks,
	"transform":          verbTransform,
	"migrate":            verbMigrate,
	"metrics":            verbMetrics,
	"score":              verbScore,
	"scorecard":          verbScorecard,
	"risk-map":           verbRiskMap,
	"blast-radius":       verbBlastRadius,
	"baseline-audit":     verbBaselineAudit,
	"suggest-reviewers":  verbSuggestReviewers,
	"editorconfig-drift": verbEditorConfigDrift,
	"gen":                verbGen,
	"graph":              verbGraph,
	"precommit":          verbPrecommit,
	"snapshot":           verbSnapshot,
	"daemon":             verbDaemon,
	"triage":             verbTriage,
	"rules":              verbRules,
	"breakage":           verbBreakage,
	"bisect-structure":   verbBisectStructure,
	"delta-risk":         verbDeltaRisk,
	"traces":             verbTraces,
}

func classifyVerb(arg string) subcommandVerb {
	if v, ok := verbByName[arg]; ok {
		return v
	}
	return verbNone
}

// runVerbAndExit runs the subcommand for the given verb and exits. Returns
// false if the verb is not handled here (caller should try other paths).
func runVerbAndExit(verb subcommandVerb, rest []string) bool {
	switch verb {
	case verbCache:
		os.Exit(climcache.Run(rest))
	case verbServe:
		os.Exit(climserve.Run(rest))
	case verbHarvest:
		os.Exit(climharvest.Run(rest))
	case verbRename:
		os.Exit(climrename.Run(rest))
	case verbInit:
		os.Exit(climinit.Run(rest))
	case verbAPISnapshot:
		os.Exit(climapi.RunSnapshot(rest))
	case verbAPIDiff:
		os.Exit(climapi.RunDiff(rest))
	case verbABIHash:
		os.Exit(climabihash.Run(rest))
	case verbImpact:
		os.Exit(climimpact.Run(rest))
	case verbDeadCode:
		os.Exit(climdeadcode.Run(rest))
	case verbDIGraph:
		os.Exit(climdigraph.Run(rest))
	case verbUsedSymbols:
		os.Exit(climusedsymbols.Run(rest))
	case verbTestCoverage:
		os.Exit(climtestcoverage.Run(rest))
	case verbSelectTests:
		os.Exit(climselecttests.Run(rest))
	case verbMocks:
		os.Exit(climmocks.Run(rest))
	default:
		return runVerbAndExitB(verb, rest)
	}
	return true
}

func runVerbAndExitB(verb subcommandVerb, rest []string) bool {
	switch verb {
	case verbTransform:
		os.Exit(climtransform.Run(rest))
	case verbMigrate:
		os.Exit(climmigrate.Run(rest))
	case verbMetrics:
		os.Exit(climmetrics.Run(rest, version))
	case verbScore:
		os.Exit(climscore.Run(rest))
	case verbScorecard:
		os.Exit(climscorecard.Run(rest))
	case verbRiskMap:
		os.Exit(climriskmap.Run(rest))
	case verbBlastRadius:
		os.Exit(climblastradius.Run(rest))
	case verbSuggestReviewers:
		os.Exit(climsuggestreviewers.Run(rest))
	case verbEditorConfigDrift:
		os.Exit(climeditorconfigdrift.Run(rest))
	case verbGen:
		os.Exit(climgen.Run(rest))
	case verbGraph:
		os.Exit(climgraph.Run(rest))
	case verbPrecommit:
		os.Exit(climprecommit.Run(rest))
	case verbSnapshot:
		climsnapshot.Version = version
		os.Exit(climsnapshot.Run(rest))
	case verbDaemon:
		os.Exit(climdaemoncmd.Run(rest))
	case verbTriage:
		os.Exit(climtriage.Run(rest))
	case verbRules:
		os.Exit(climrules.Run(rest))
	case verbBreakage:
		os.Exit(climbreakage.Run(rest))
	case verbBisectStructure:
		os.Exit(climbisect.Run(rest))
	case verbDeltaRisk:
		os.Exit(climdeltarisk.Run(rest))
	default:
		return runVerbAndExitC(verb, rest)
	}
	return false
}

// runVerbAndExitC is the third dispatch tier. Verbs added after the
// per-function cyclo limit was reached should land here so the
// existing tiers don't need to be reshuffled when more subcommands
// arrive.
func runVerbAndExitC(verb subcommandVerb, rest []string) bool {
	switch verb {
	case verbTraces:
		os.Exit(climtraces.Run(rest))
	}
	return false
}

// dispatchSubcommand inspects os.Args[1] for a recognized subcommand verb.
// For verbs that own their own CLI (cache, serve, harvest, etc.) it runs the
// subcommand and terminates the process via os.Exit.
// For "baseline-audit" it rewrites os.Args to drop the verb and returns true
// so the standard scan flow can proceed with --baseline-audit semantics.
// Returns false when there is no recognized verb.
func dispatchSubcommand() bool {
	if len(os.Args) <= 1 {
		return false
	}
	rest := os.Args[2:]
	verb := classifyVerb(os.Args[1])
	if verb == verbBaselineAudit {
		os.Args = append([]string{os.Args[0]}, rest...)
		return true
	}
	return runVerbAndExit(verb, rest)
}
