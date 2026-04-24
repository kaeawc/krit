package firchecks

import "github.com/kaeawc/krit/internal/scanner"

// FirChecker is the interface for running FIR checks. The production
// implementation calls InvokeCached; tests use FakeFirChecker.
type FirChecker interface {
	Check(files []string, sourceDirs, classpath, rules []string) (*Result, error)
}

// FakeFirChecker is a configurable test double for FirChecker.
// Set Findings and Crashed before use.
type FakeFirChecker struct {
	Findings []scanner.Finding
	Crashed  map[string]string
	Err      error
	// Called is the list of file slices passed to Check.
	Called [][]string
}

// NewFakeFirChecker returns a FakeFirChecker with all maps initialized.
func NewFakeFirChecker() *FakeFirChecker {
	return &FakeFirChecker{
		Crashed: map[string]string{},
	}
}

// Check records the call and returns the configured findings.
func (f *FakeFirChecker) Check(files []string, sourceDirs, classpath, rules []string) (*Result, error) {
	cp := make([]string, len(files))
	copy(cp, files)
	f.Called = append(f.Called, cp)
	if f.Err != nil {
		return nil, f.Err
	}
	crashed := f.Crashed
	if crashed == nil {
		crashed = map[string]string{}
	}
	return &Result{
		Findings: append([]scanner.Finding(nil), f.Findings...),
		Crashed:  crashed,
	}, nil
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
func (p *ProductionFirChecker) Check(files []string, sourceDirs, classpath, rules []string) (*Result, error) {
	sd := p.SourceDirs
	if len(sourceDirs) > 0 {
		sd = sourceDirs
	}
	cl := p.Classpath
	if len(classpath) > 0 {
		cl = classpath
	}
	return InvokeCached(p.JarPath, files, sd, cl, rules, p.RepoDir, p.UseDaemon, p.Verbose)
}

// Compile-time check.
var _ FirChecker = (*ProductionFirChecker)(nil)
