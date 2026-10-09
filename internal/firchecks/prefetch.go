package firchecks

// prefetch.go — reuse the check response the oracle's krit-fir compilation
// produced for this scan's FIR pass (#739).

import (
	"encoding/json"
	"slices"
	"sync"

	"github.com/kaeawc/krit/internal/oracle"
)

// prefetchRequestID is the request ID a prefetched check is sent with.
const prefetchRequestID = 1

// Prefetch carries the FIR pass's check request onto the oracle's krit-fir
// compilation and holds the response, so a --fir scan compiles the module
// once instead of once for the oracle and again for the checkers.
//
// The pass reuses the response only for a request that would have compiled
// the same thing: the same jar, compile context, rules and options, the
// same facts for every file it checks, a subset of the prefetched files, and
// a compilation (every source and requested file, by path and content)
// unchanged since the request rode on the oracle. Anything else runs the
// pass's own check, as before.
type Prefetch struct {
	checker *ProductionFirChecker
	opts    PassOptions

	mu          sync.Mutex
	plan        passPlan
	files       map[string]bool
	configs     string
	compilation string
	resp        *CheckResponse
}

// NewPrefetch returns a prefetch for the pass opts describes: the pass the
// scan will start after parsing, built from the collected Kotlin paths. Nil
// when opts.Checker is not a *ProductionFirChecker with a jar, or the pass
// is disabled. The request is planned only when a rider is prepared.
func NewPrefetch(opts PassOptions) *Prefetch {
	checker, ok := opts.Checker.(*ProductionFirChecker)
	if !opts.Enabled || !ok || checker == nil || checker.JarPath == "" {
		return nil
	}
	return &Prefetch{checker: checker, opts: opts}
}

// Rider returns the rider for an oracle compilation of jarPath over
// sourceDirs and classpath for jvmTarget, or nil when that is not the
// checker's own compile context.
func (p *Prefetch) Rider(jarPath string, sourceDirs, classpath []string, jvmTarget string) *oracle.CheckRider {
	if p == nil {
		return nil
	}
	c := p.checker
	if jarPath != c.JarPath || jvmTarget != c.JvmTarget || !sameAbsolutePaths(sourceDirs, c.SourceDirs) || !sameAbsolutePaths(classpath, c.Classpath) {
		return nil
	}
	return &oracle.CheckRider{Prepare: p.prepare}
}

// prepare plans the pass and returns its check request, recording what the
// compile will see so the pass can tell whether the response still applies.
func (p *Prefetch) prepare() ([]byte, func([]byte)) {
	plan, ok := planPass(p.opts)
	if !ok || len(plan.requested) == 0 {
		return nil, nil
	}
	c := p.checker
	refs := make([]fileRef, len(plan.requested))
	files := make(map[string]bool, len(plan.requested))
	for i, path := range plan.requested {
		refs[i] = fileRef{Path: path}
		files[path] = true
	}
	request, spelling, err := encodeCheckRequest(prefetchRequestID, refs, c.SourceDirs, c.Classpath, plan.rules, plan.configs, plan.facts, c.JvmTarget)
	if err != nil {
		return nil, nil
	}
	// Taken before the compile, so an edit made while the oracle runs
	// leaves the response unused.
	compilation := checkCompilation(c.SourceDirs, plan.requested, c.Classpath, c.JarPath)
	p.mu.Lock()
	p.plan, p.files, p.configs, p.compilation, p.resp = plan, files, configsKey(plan.configs), compilation, nil
	p.mu.Unlock()
	return request, func(data []byte) {
		resp, err := decodeCheckResponse(data, prefetchRequestID, spelling)
		if err != nil || missingKnownFIRRules(plan.rules, resp) {
			// An AOT-poisoned JVM registers no checkers; the pass's own
			// check recovers from that.
			return
		}
		p.mu.Lock()
		if p.compilation == compilation {
			p.resp = resp
		}
		p.mu.Unlock()
	}
}

// response returns the prefetched response when a check of files with these
// inputs would compile exactly what the prefetched one did.
func (p *Prefetch) response(jarPath, jvmTarget string, files, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts) (*CheckResponse, bool) {
	if p == nil {
		return nil, false
	}
	p.mu.Lock()
	resp, plan, planned, configs, compilation := p.resp, p.plan, p.files, p.configs, p.compilation
	p.mu.Unlock()
	c := p.checker
	if resp == nil || jarPath != c.JarPath || jvmTarget != c.JvmTarget ||
		!sameAbsolutePaths(sourceDirs, c.SourceDirs) || !sameAbsolutePaths(classpath, c.Classpath) ||
		!sameRuleSet(rules, plan.rules) || configsKey(ruleConfigs) != configs {
		return nil, false
	}
	for _, path := range files {
		if !planned[path] {
			return nil, false
		}
	}
	if !sameFacts(facts.forFiles(files), plan.facts.forFiles(files), files) {
		return nil, false
	}
	if checkCompilation(sourceDirs, files, classpath, jarPath) != compilation {
		return nil, false
	}
	return resp, true
}

func sameAbsolutePaths(a, b []string) bool {
	return slices.Equal(oracle.AbsolutePaths(a), oracle.AbsolutePaths(b))
}

func sameRuleSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

// configsKey is the options encoding the cache fingerprint uses.
func configsKey(configs RuleConfigs) string {
	data, err := json.Marshal(wireRuleConfigs(configs))
	if err != nil {
		return "unencodable"
	}
	return string(data)
}

// sameFacts reports whether a and b say the same about every file in files.
func sameFacts(a, b FileFacts, files []string) bool {
	testA, testB := map[string]bool{}, map[string]bool{}
	for _, path := range a.TestFiles {
		testA[path] = true
	}
	for _, path := range b.TestFiles {
		testB[path] = true
	}
	for _, path := range files {
		if testA[path] != testB[path] || a.ScanPaths[path] != b.ScanPaths[path] || a.SDKLevels[path] != b.SDKLevels[path] {
			return false
		}
	}
	return true
}

// forFiles returns the part of r about files: their findings, crashes,
// error files, and rule errors. Every per-file verdict comes from the same
// whole-module compilation whatever else was requested, so the part answers
// a check of just those files.
func (r *CheckResponse) forFiles(files []string) *CheckResponse {
	want := make(map[string]bool, len(files))
	for _, path := range files {
		want[path] = true
	}
	out := &CheckResponse{ID: r.ID, Rules: slices.Clone(r.Rules), rulesPresent: r.rulesPresent}
	for _, f := range r.Findings {
		if want[f.Path] {
			out.Findings = append(out.Findings, f)
		}
	}
	out.Crashed = keepPaths(r.Crashed, want)
	out.ErrorFiles = keepPaths(r.ErrorFiles, want)
	for rule, byPath := range r.RuleErrors {
		if kept := keepPaths(byPath, want); len(kept) > 0 {
			if out.RuleErrors == nil {
				out.RuleErrors = map[string]map[string]string{}
			}
			out.RuleErrors[rule] = kept
		}
	}
	for _, path := range files {
		if _, crashed := out.Crashed[path]; !crashed {
			if _, gated := out.ErrorFiles[path]; !gated {
				out.Succeeded++
			}
		}
	}
	return out
}

func keepPaths(byPath map[string]string, want map[string]bool) map[string]string {
	var out map[string]string
	for path, value := range byPath {
		if want[path] {
			if out == nil {
				out = map[string]string{}
			}
			out[path] = value
		}
	}
	return out
}
