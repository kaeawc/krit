// Package lazyre provides regular expressions that compile on first use.
//
// Package-level regexp.MustCompile calls run during program initialization,
// so every krit invocation (including --version, LSP and MCP startup, and
// scans whose rules never reach a given pattern) pays for compiling all of
// them. A *Regexp declared with New compiles its pattern the first time a
// method is called and then behaves exactly like the compiled
// *regexp.Regexp.
package lazyre

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
)

// Regexp is a regular expression compiled on first use. It is safe for
// concurrent use.
type Regexp struct {
	expr string
	once sync.Once
	re   atomic.Pointer[regexp.Regexp]
}

var (
	registryMu sync.Mutex
	registry   []*Regexp
)

// New returns a Regexp for expr. Like regexp.MustCompile it panics when expr
// does not compile, but on first use instead of at initialization; CompileAll
// lets tests check every declared pattern up front.
func New(expr string) *Regexp {
	r := &Regexp{expr: expr}
	registryMu.Lock()
	registry = append(registry, r)
	registryMu.Unlock()
	return r
}

// Re returns the compiled regular expression.
func (r *Regexp) Re() *regexp.Regexp {
	if re := r.re.Load(); re != nil {
		return re
	}
	return r.compile()
}

func (r *Regexp) compile() *regexp.Regexp {
	r.once.Do(func() { r.re.Store(regexp.MustCompile(r.expr)) })
	return r.re.Load()
}

// CompileAll compiles every Regexp created with New so far and reports the
// first pattern that does not compile.
func CompileAll() error {
	registryMu.Lock()
	all := append([]*Regexp(nil), registry...)
	registryMu.Unlock()
	for _, r := range all {
		if _, err := regexp.Compile(r.expr); err != nil {
			return fmt.Errorf("lazyre: %q: %w", r.expr, err)
		}
	}
	return nil
}

// Count reports how many Regexps have been created with New.
func Count() int {
	registryMu.Lock()
	defer registryMu.Unlock()
	return len(registry)
}

// The methods below forward to the compiled *regexp.Regexp.

func (r *Regexp) String() string { return r.expr }

func (r *Regexp) Match(b []byte) bool                    { return r.Re().Match(b) }
func (r *Regexp) MatchString(s string) bool              { return r.Re().MatchString(s) }
func (r *Regexp) Find(b []byte) []byte                   { return r.Re().Find(b) }
func (r *Regexp) FindIndex(b []byte) []int               { return r.Re().FindIndex(b) }
func (r *Regexp) FindString(s string) string             { return r.Re().FindString(s) }
func (r *Regexp) FindStringIndex(s string) []int         { return r.Re().FindStringIndex(s) }
func (r *Regexp) FindSubmatch(b []byte) [][]byte         { return r.Re().FindSubmatch(b) }
func (r *Regexp) FindSubmatchIndex(b []byte) []int       { return r.Re().FindSubmatchIndex(b) }
func (r *Regexp) FindStringSubmatch(s string) []string   { return r.Re().FindStringSubmatch(s) }
func (r *Regexp) FindStringSubmatchIndex(s string) []int { return r.Re().FindStringSubmatchIndex(s) }
func (r *Regexp) FindAll(b []byte, n int) [][]byte       { return r.Re().FindAll(b, n) }
func (r *Regexp) FindAllIndex(b []byte, n int) [][]int   { return r.Re().FindAllIndex(b, n) }
func (r *Regexp) FindAllString(s string, n int) []string {
	return r.Re().FindAllString(s, n)
}
func (r *Regexp) FindAllStringIndex(s string, n int) [][]int {
	return r.Re().FindAllStringIndex(s, n)
}
func (r *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	return r.Re().FindAllSubmatch(b, n)
}
func (r *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	return r.Re().FindAllSubmatchIndex(b, n)
}
func (r *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	return r.Re().FindAllStringSubmatch(s, n)
}
func (r *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	return r.Re().FindAllStringSubmatchIndex(s, n)
}
func (r *Regexp) ReplaceAllString(src, repl string) string {
	return r.Re().ReplaceAllString(src, repl)
}
func (r *Regexp) ReplaceAllLiteralString(src, repl string) string {
	return r.Re().ReplaceAllLiteralString(src, repl)
}
func (r *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	return r.Re().ReplaceAllStringFunc(src, repl)
}
func (r *Regexp) ReplaceAll(src, repl []byte) []byte { return r.Re().ReplaceAll(src, repl) }
func (r *Regexp) Split(s string, n int) []string     { return r.Re().Split(s, n) }
func (r *Regexp) SubexpNames() []string              { return r.Re().SubexpNames() }
func (r *Regexp) SubexpIndex(name string) int        { return r.Re().SubexpIndex(name) }
func (r *Regexp) NumSubexp() int                     { return r.Re().NumSubexp() }
