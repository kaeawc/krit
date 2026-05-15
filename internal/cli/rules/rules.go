// Package rules implements the `krit rules` CLI verb. It exposes
// list and coverage subcommands that query the rule registry by
// language support classification.
package rules

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	rulespkg "github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// Run is the verb entrypoint wired from cmd/krit's verb dispatcher.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "rules: missing subcommand; expected 'list' or 'coverage'")
		return 2
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		return runList(rest, stdout, stderr)
	case "coverage":
		return runCoverage(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "rules: unknown subcommand %q; expected 'list' or 'coverage'\n", sub)
		return 2
	}
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	language := fs.String("language", "", "Restrict to a language ('java'); when set, --status filters by LanguageSupport classification")
	status := fs.String("status", "", "LanguageSupport status filter (comma-separated, e.g. 'partial,pending' or '!supported')")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	filter, err := buildFilter(*language, *status)
	if err != nil {
		fmt.Fprintf(stderr, "rules list: %v\n", err)
		return 2
	}
	matches := rulespkg.FilterRegistry(api.Registry, filter)
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if filter.Language != "" {
		fmt.Fprintln(tw, "RULE\tCATEGORY\tSEVERITY\tSTATUS\tREASON/EVIDENCE")
	} else {
		fmt.Fprintln(tw, "RULE\tCATEGORY\tSEVERITY")
	}
	for _, r := range matches {
		if filter.Language == "" {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.ID, r.Category, string(r.Sev))
			continue
		}
		statusStr, note := supportCell(r, filter.Language)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Category, string(r.Sev), statusStr, note)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(stderr, "rules list: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "\n%d rule(s)\n", len(matches))
	return 0
}

func runCoverage(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules coverage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	language := fs.String("language", "java", "Language to report coverage for (currently only 'java' is wired)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *language == "" {
		fmt.Fprintln(stderr, "rules coverage: --language is required")
		return 2
	}
	matches := append([]*api.Rule(nil), api.Registry...)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i] == nil || matches[j] == nil {
			return matches[i] != nil
		}
		return matches[i].ID < matches[j].ID
	})

	totals := map[api.LanguageSupportStatus]int{}
	unclassified := 0

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "RULE\tCATEGORY\t%s STATUS\tREASON/EVIDENCE\n", strings.ToUpper(*language))
	for _, r := range matches {
		if r == nil {
			continue
		}
		statusStr, note := supportCell(r, *language)
		if support, ok := rulespkg.LanguageSupportForRule(r, *language); ok {
			totals[support.Status]++
		} else {
			unclassified++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Category, statusStr, note)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(stderr, "rules coverage: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Total: %d rule(s)\n", len(matches))
	for _, status := range []api.LanguageSupportStatus{
		api.LanguageSupportSupported,
		api.LanguageSupportPartial,
		api.LanguageSupportPending,
		api.LanguageSupportNeedsDesign,
		api.LanguageSupportNotApplicable,
	} {
		fmt.Fprintf(stdout, "  %s: %d\n", status, totals[status])
	}
	if unclassified > 0 {
		fmt.Fprintf(stdout, "  (unclassified): %d\n", unclassified)
	}
	return 0
}

// supportCell returns the (status, note) pair to render for a rule in the
// LanguageSupport column. Unclassified rules return a sentinel placeholder.
func supportCell(r *api.Rule, language string) (string, string) {
	support, ok := rulespkg.LanguageSupportForRule(r, language)
	if !ok {
		return "(unclassified)", ""
	}
	return string(support.Status), support.Summary()
}

func buildFilter(language, statusExpr string) (rulespkg.LanguageSupportFilter, error) {
	statuses, negate, err := rulespkg.ParseStatusFilter(statusExpr)
	if err != nil {
		return rulespkg.LanguageSupportFilter{}, err
	}
	if language == "" && (len(statuses) > 0 || negate) {
		return rulespkg.LanguageSupportFilter{}, fmt.Errorf("--status requires --language")
	}
	filter := rulespkg.LanguageSupportFilter{
		Language: language,
		Status:   statuses,
		Negate:   negate,
	}
	if err := filter.Validate(); err != nil {
		return rulespkg.LanguageSupportFilter{}, err
	}
	return filter, nil
}
