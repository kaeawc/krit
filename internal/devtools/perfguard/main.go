// Command perfguard is krit's CI performance regression guard (#1657).
//
//	perfguard gen -out DIR [-modules N -files N -classes N]
//	    Write a deterministic synthetic Android-shaped project.
//	perfguard compare -base BIN -head BIN -corpus NAME=DIR [-corpus ...]
//	    Time both krit binaries on each corpus in interleaved rounds on the
//	    same machine and fail when head's median total, a phase or a rule is
//	    slower than the gates allow. No stored baseline: a shared CI runner's
//	    speed varies run to run, so base and head are measured side by side.
//	perfguard scale -bin BIN -small DIR -large DIR -factor F
//	    Time one binary on a small and a large synthetic corpus and fail when
//	    a phase or rule grows faster than the corpus (super-linear), unless
//	    known-superlinear.txt lists it with its issue.
//
// Both report as markdown (-report FILE, for the job summary or a PR
// comment) and exit 1 on a regression.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	var failed bool
	switch os.Args[1] {
	case "gen":
		err = cmdGen(os.Args[2:])
	case "compare":
		failed, err = cmdCompare(os.Args[2:])
	case "scale":
		failed, err = cmdScale(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "perfguard:", err)
		os.Exit(2)
	}
	if failed {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: perfguard gen|compare|scale [flags] (see -h on each)")
	os.Exit(2)
}

func cmdGen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	out := fs.String("out", "", "output directory (required)")
	spec := genSpec{}
	fs.IntVar(&spec.Modules, "modules", 20, "Gradle feature modules")
	fs.IntVar(&spec.FilesPerModule, "files", 20, "Kotlin files per module")
	fs.IntVar(&spec.ClassesPerFile, "classes", 6, "declarations per file")
	_ = fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("gen: -out is required")
	}
	if err := generate(*out, spec); err != nil {
		return err
	}
	fmt.Printf("generated %s: %d modules, %d files, %d declarations\n", *out, spec.Modules, spec.Modules*spec.FilesPerModule, spec.classes())
	return nil
}

type corpusFlags []string

func (c *corpusFlags) String() string     { return strings.Join(*c, ",") }
func (c *corpusFlags) Set(v string) error { *c = append(*c, v); return nil }

func scanArgs(extra string) []string {
	args := append([]string{}, defaultScanArgs...)
	if extra != "" {
		args = append(args, strings.Fields(extra)...)
	}
	return args
}

func openReport(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return io.MultiWriter(f, os.Stdout), func() { _ = f.Close() }, nil
}

func logf(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }

func cmdCompare(args []string) (bool, error) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	base := fs.String("base", "", "base krit binary (required)")
	head := fs.String("head", "", "head krit binary (required)")
	var corpora corpusFlags
	fs.Var(&corpora, "corpus", "NAME=DIR to scan (repeatable)")
	rounds := fs.Int("rounds", 5, "interleaved base/head rounds per corpus")
	extra := fs.String("args", "", "extra krit arguments")
	report := fs.String("report", "", "also write the markdown report here")
	title := fs.String("title", "Performance: base vs head", "report heading")
	g := defaultGates
	fs.Float64Var(&g.TotalPct, "total-pct", g.TotalPct, "total regression gate, percent")
	fs.Float64Var(&g.TotalMinMs, "total-min-ms", g.TotalMinMs, "total regression gate, milliseconds")
	fs.Float64Var(&g.PhasePct, "phase-pct", g.PhasePct, "phase regression gate, percent")
	fs.Float64Var(&g.PhaseMinMs, "phase-min-ms", g.PhaseMinMs, "phase regression gate, milliseconds")
	fs.Float64Var(&g.RulePct, "rule-pct", g.RulePct, "rule regression gate, percent")
	fs.Float64Var(&g.RuleMinMs, "rule-min-ms", g.RuleMinMs, "rule regression gate, milliseconds")
	fs.Float64Var(&g.MaxRuleShare, "max-rule-share", g.MaxRuleShare, "fail a rule that newly exceeds this share of rule time, percent")
	fs.Float64Var(&g.NewRuleMaxMs, "new-rule-max-ms", g.NewRuleMaxMs, "fail a rule new in head that costs this much")
	_ = fs.Parse(args)
	if *base == "" || *head == "" || len(corpora) == 0 || *rounds < 1 {
		return false, fmt.Errorf("compare: -base, -head, at least one -corpus and -rounds >= 1 are required")
	}
	ctx := context.Background()
	var results []corpusResult
	var regs []regression
	for _, c := range corpora {
		name, dir, ok := strings.Cut(c, "=")
		if !ok {
			name, dir = c, c
		}
		logf("perfguard: %s (%s), %d rounds", name, dir, *rounds)
		bs, hs, err := measureInterleaved(ctx, *base, *head, scanArgs(*extra), dir, *rounds, logf)
		if err != nil {
			return false, err
		}
		res := corpusResult{Name: name, Base: summarize(bs), Head: summarize(hs), Rounds: *rounds}
		results = append(results, res)
		regs = append(regs, compareSummaries(name, res.Base, res.Head, g)...)
	}
	w, closeFn, err := openReport(*report)
	if err != nil {
		return false, err
	}
	defer closeFn()
	writeReport(w, results, regs, g, *title)
	return len(regs) > 0, nil
}

func cmdScale(args []string) (bool, error) {
	fs := flag.NewFlagSet("scale", flag.ExitOnError)
	bin := fs.String("bin", "", "krit binary (required)")
	baseBin := fs.String("base", "", "base krit binary: metrics it scales as badly don't fail")
	small := fs.String("small", "", "small corpus directory (required)")
	large := fs.String("large", "", "large corpus directory (required)")
	g := defaultScaleGates
	fs.Float64Var(&g.Factor, "factor", 0, "how many times larger the large corpus is (required)")
	fs.Float64Var(&g.Slack, "slack", g.Slack, "allowed growth over linear")
	fs.Float64Var(&g.MinMs, "min-ms", g.MinMs, "ignore metrics below this on the large corpus")
	rounds := fs.Int("rounds", 3, "runs per corpus (median)")
	extra := fs.String("args", "", "extra krit arguments")
	allow := fs.String("known", "", "known-superlinear allowlist file")
	report := fs.String("report", "", "also write the markdown report here")
	_ = fs.Parse(args)
	if *bin == "" || *small == "" || *large == "" || g.Factor <= 1 || *rounds < 1 {
		return false, fmt.Errorf("scale: -bin, -small, -large, -factor > 1 and -rounds >= 1 are required")
	}
	known, err := loadKnown(*allow)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	logf("perfguard: scaling %s -> %s, %d rounds each", *small, *large, *rounds)
	ss, err := measure(ctx, *bin, scanArgs(*extra), *small, *rounds)
	if err != nil {
		return false, err
	}
	ls, err := measure(ctx, *bin, scanArgs(*extra), *large, *rounds)
	if err != nil {
		return false, err
	}
	results, stale := checkScaling(summarize(ss), summarize(ls), g, known)
	if *baseBin != "" {
		logf("perfguard: scaling the base binary too")
		bss, err := measure(ctx, *baseBin, scanArgs(*extra), *small, *rounds)
		if err != nil {
			return false, err
		}
		bls, err := measure(ctx, *baseBin, scanArgs(*extra), *large, *rounds)
		if err != nil {
			return false, err
		}
		baseResults, _ := checkScaling(summarize(bss), summarize(bls), g, knownSuperlinear{})
		results = applyBase(results, baseResults)
	}
	w, closeFn, err := openReport(*report)
	if err != nil {
		return false, err
	}
	defer closeFn()
	return writeScaleReport(w, *small, *large, results, stale, g, known), nil
}
