package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func runKotlinPluginRulesAndMerge(ctx context.Context, args ProjectArgs, host ProjectHostState, indexResult IndexResult, crossFileResult *CrossFileResult, bundleHit bool) error {
	if len(args.CustomRuleJars) == 0 || bundleHit {
		return nil
	}
	daemon := indexResult.Daemon
	if daemon == nil {
		daemon = host.OracleDaemon
	}
	if daemon == nil {
		return fmt.Errorf("kotlin custom rules require the krit-types daemon; pass --daemon. Released krit binaries auto-download the matching krit-types.jar into ~/.krit/jars/ on first use; for dev builds run: cd tools/krit-types && ./gradlew shadowJar (or set KRIT_TYPES_JAR to an existing jar)")
	}

	list, err := daemon.ListPlugins(args.CustomRuleJars)
	if err != nil {
		return fmt.Errorf("list Kotlin custom rules: %w", err)
	}
	ruleIDs, ruleOptions := selectPluginRules(list.Rules, args.Config)
	if len(ruleIDs) == 0 {
		return nil
	}

	collector := scanner.NewFindingCollector(crossFileResult.Findings.Len())
	collector.AppendColumns(&crossFileResult.Findings)
	for _, file := range indexResult.KotlinFiles {
		if file == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		result, err := daemon.AnalyzePluginFile(args.CustomRuleJars, file.Path, file.Content, ruleIDs, ruleOptions)
		if err != nil {
			return fmt.Errorf("run Kotlin custom rules on %s: %w", file.Path, err)
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("run Kotlin custom rules on %s: %s", file.Path, formatPluginErrors(result.Errors))
		}
		cols := pluginFindingsToColumns(result.Findings)
		cols = applySuppressionColumns(&cols, indexResult.SourceFiles())
		collector.AppendColumns(&cols)
	}
	crossFileResult.Findings = *collector.Columns()
	return nil
}

// selectPluginRules picks which jar-loaded rules to dispatch and collects
// each rule's configured options from `pluginRules:` in krit.yml. A rule
// is skipped when the user explicitly set `active: false`; the absence of
// any pluginRules entry preserves the daemon's default activation.
func selectPluginRules(loaded []oracle.PluginRuleDescriptor, cfg *config.Config) ([]string, map[string]map[string]interface{}) {
	ruleIDs := make([]string, 0, len(loaded))
	options := map[string]map[string]interface{}{}
	for _, rule := range loaded {
		if rule.RuleID == "" {
			continue
		}
		if active := cfg.IsPluginRuleActive(rule.RuleID); active != nil && !*active {
			continue
		}
		ruleIDs = append(ruleIDs, rule.RuleID)
		if opts := cfg.PluginRuleOptions(rule.RuleID); len(opts) > 0 {
			options[rule.RuleID] = opts
		}
	}
	return ruleIDs, options
}

func pluginFindingsToColumns(findings []oracle.PluginFinding) scanner.FindingColumns {
	if len(findings) == 0 {
		return scanner.FindingColumns{}
	}
	out := make([]scanner.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, scanner.Finding{
			File:       finding.File,
			Line:       finding.Line,
			Col:        finding.Column,
			StartByte:  finding.StartByte,
			EndByte:    finding.EndByte,
			RuleSet:    finding.RuleSet,
			Rule:       finding.RuleID,
			Severity:   finding.Severity,
			Message:    finding.Message,
			Confidence: finding.Confidence,
			Fix:        pluginFixToScanner(finding.Fix),
		})
	}
	return scanner.CollectFindings(out)
}

func pluginFixToScanner(fix *oracle.PluginFix) *scanner.Fix {
	if fix == nil {
		return nil
	}
	out := &scanner.Fix{
		StartLine:   fix.StartLine,
		EndLine:     fix.EndLine,
		Replacement: fix.Replacement,
	}
	if lvl, ok := rules.ParseFixLevel(fix.Safety); ok {
		out.Safety = uint8(lvl)
	} else {
		// Unknown/missing → semantic so --fix-level still gates it.
		out.Safety = uint8(rules.FixSemantic)
	}
	return out
}

func formatPluginErrors(errors map[string]string) string {
	parts := make([]string, 0, len(errors))
	for ruleID, message := range errors {
		parts = append(parts, ruleID+": "+message)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
