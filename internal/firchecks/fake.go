package firchecks

import (
	"slices"

	"github.com/kaeawc/krit/internal/scanner"
)

// FirChecker is the interface for running FIR checks. The production
// implementation calls InvokeCached; tests use FakeFirChecker.
type FirChecker interface {
	Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, testFiles []string) (*Result, error)
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
	// CalledTestFiles is the testFiles passed to each Check call.
	CalledTestFiles [][]string
}

// NewFakeFirChecker returns a FakeFirChecker with all maps initialized.
func NewFakeFirChecker() *FakeFirChecker {
	return &FakeFirChecker{
		Crashed:    map[string]string{},
		ErrorFiles: map[string]string{},
	}
}

// Check records the call and returns the configured findings.
func (f *FakeFirChecker) Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, testFiles []string) (*Result, error) {
	f.Called = append(f.Called, slices.Clone(files))
	f.CalledSourceDirs = append(f.CalledSourceDirs, slices.Clone(sourceDirs))
	f.CalledClasspath = append(f.CalledClasspath, slices.Clone(classpath))
	f.CalledRules = append(f.CalledRules, slices.Clone(rules))
	f.CalledRuleConfigs = append(f.CalledRuleConfigs, ruleConfigs)
	f.CalledTestFiles = append(f.CalledTestFiles, slices.Clone(testFiles))
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
	JarPath    string
	SourceDirs []string
	Classpath  []string
	RepoDir    string
	UseDaemon  bool
	Verbose    bool
}

// Check runs InvokeCached with the configured parameters.
func (p *ProductionFirChecker) Check(files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, testFiles []string) (*Result, error) {
	sd := p.SourceDirs
	if len(sourceDirs) > 0 {
		sd = sourceDirs
	}
	cl := p.Classpath
	if len(classpath) > 0 {
		cl = classpath
	}
	return InvokeCached(p.JarPath, files, sd, cl, rules, ruleConfigs, testFiles, p.RepoDir, p.UseDaemon, p.Verbose)
}

// Compile-time check.
var _ FirChecker = (*ProductionFirChecker)(nil)
