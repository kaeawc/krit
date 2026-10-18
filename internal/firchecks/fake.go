package firchecks

import (
	"maps"
	"slices"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/scanner"
)

// FirChecker is the interface for running FIR checks. The production
// implementation calls InvokeCached; tests use FakeFirChecker.
type FirChecker interface {
	Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts) (*Result, error)
}

// FakeFirChecker is a configurable test double for FirChecker.
// Set Findings, Crashed, ErrorFiles, RuleErrors, and Rules before use.
type FakeFirChecker struct {
	Findings   []scanner.Finding
	Crashed    map[string]string
	ErrorFiles map[string]string
	// RuleErrors is rule ID -> file -> checker exception message.
	RuleErrors map[string]map[string]string
	// Rules is the advertised rule set. nil advertises every requested rule.
	Rules []string
	Err   error
	// Called is the list of file slices passed to Check.
	Called [][]string
	// CalledSourceDirs / CalledClasspath / CalledRules record each call's
	// compile context and requested rules.
	CalledSourceDirs [][]string
	CalledClasspath  [][]string
	CalledRules      [][]string
	// CalledRuleConfigs is the ruleConfigs passed to each Check call.
	CalledRuleConfigs []RuleConfigs
	// CalledTestFiles is the facts.TestFiles passed to each Check call.
	CalledTestFiles [][]string
	// CalledScanPaths is the facts.ScanPaths passed to each Check call.
	CalledScanPaths []map[string]string
	// CalledSDKLevels is the facts.SDKLevels passed to each Check call.
	CalledSDKLevels []map[string]android.SDKLevels
}

// NewFakeFirChecker returns a FakeFirChecker with all maps initialized.
func NewFakeFirChecker() *FakeFirChecker {
	return &FakeFirChecker{
		Crashed:    map[string]string{},
		ErrorFiles: map[string]string{},
	}
}

// Check records the call and returns the configured findings.
func (f *FakeFirChecker) Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts) (*Result, error) {
	f.Called = append(f.Called, slices.Clone(files))
	f.CalledSourceDirs = append(f.CalledSourceDirs, slices.Clone(sourceDirs))
	f.CalledClasspath = append(f.CalledClasspath, slices.Clone(classpath))
	f.CalledRules = append(f.CalledRules, slices.Clone(rules))
	f.CalledRuleConfigs = append(f.CalledRuleConfigs, ruleConfigs)
	f.CalledTestFiles = append(f.CalledTestFiles, slices.Clone(facts.TestFiles))
	f.CalledScanPaths = append(f.CalledScanPaths, maps.Clone(facts.ScanPaths))
	f.CalledSDKLevels = append(f.CalledSDKLevels, maps.Clone(facts.SDKLevels))
	if f.Err != nil {
		return nil, f.Err
	}
	res := newResult()
	res.Findings = append([]scanner.Finding(nil), f.Findings...)
	for k, v := range f.Crashed {
		res.Crashed[k] = v
	}
	for k, v := range f.ErrorFiles {
		res.ErrorFiles[k] = v
	}
	for rule, byPath := range f.RuleErrors {
		for path, msg := range byPath {
			res.addRuleError(rule, path, msg)
		}
	}
	advertised := f.Rules
	if advertised == nil {
		advertised = rules
	}
	res.addRules(slices.Clone(advertised))
	return res, nil
}

// Compile-time check.
var _ FirChecker = (*FakeFirChecker)(nil)

// ProductionFirChecker wraps InvokeCached to satisfy FirChecker.
type ProductionFirChecker struct {
	JarPath   string
	JvmTarget string
	// Kotlin is the project's effective Kotlin compiler settings; the zero value
	// leaves the embedded compiler's defaults in force.
	Kotlin     gradlemodel.KotlinSettings
	SourceDirs []string
	Classpath  []string
	RepoDir    string
	NoCache    bool
	UseDaemon  bool
	Verbose    bool
	// Prefetch, when set, may hold the check response the oracle's krit-fir
	// compilation already produced for this request (#739).
	Prefetch *Prefetch
	// Broker, when set, is how a daemon-delegated scan learns whether the
	// oracle's compilation is going to carry this check (#810): a compile
	// is due only after it has resolved.
	Broker *Broker
}

// Check runs InvokeCached with the configured parameters.
func (p *ProductionFirChecker) Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts) (*Result, error) {
	sd := p.SourceDirs
	if len(sourceDirs) > 0 {
		sd = sourceDirs
	}
	cl := p.Classpath
	if len(classpath) > 0 {
		cl = classpath
	}
	repoDir := p.RepoDir
	if p.NoCache {
		repoDir = ""
	}
	run := runMissesForCache
	if p.Prefetch != nil {
		// Decided when a compile is due, not before: a check the cache
		// answers in full never waits for the oracle or reuses anything.
		run = func(jarPath string, misses, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts, useDaemon, verbose bool, target ...CompileTarget) (*CheckResponse, error) {
			if reason := p.Broker.Wait(); p.Verbose && reason != "" {
				reporter().Verbosef("verbose: fir check: oracle compile finished (%s)\n", reason)
			}
			if resp, ok := p.Prefetch.response(p.JarPath, p.CompileTarget(), files, sd, cl, rules, ruleConfigs, facts); ok {
				if p.Verbose {
					reporter().Verbosef("verbose: fir check: reusing the oracle compilation's check of %d files\n", len(files))
				}
				return resp.forFiles(misses), nil
			}
			return runMissesForCache(jarPath, misses, sourceDirs, classpath, rules, ruleConfigs, facts, useDaemon, verbose, target...)
		}
	}
	return invokeCachedWith(run, p.JarPath, files, sd, cl, rules, ruleConfigs, facts, repoDir, p.UseDaemon, p.Verbose, p.CompileTarget())
}

// CompileTarget is the checker's JVM target and Kotlin settings as one value.
func (p *ProductionFirChecker) CompileTarget() CompileTarget {
	return CompileTarget{JvmTarget: p.JvmTarget, Kotlin: p.Kotlin}
}

// Compile-time check.
var _ FirChecker = (*ProductionFirChecker)(nil)
