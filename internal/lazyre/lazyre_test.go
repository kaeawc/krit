package lazyre

import (
	"regexp"
	"slices"
	"sync"
	"testing"
)

func TestRegexpMatchesEagerRegexp(t *testing.T) {
	const expr = `(?m)^\s*import\s+([\w.]+)(?:\s+as\s+(\w+))?$`
	lazy, eager := New(expr), regexp.MustCompile(expr)
	src := "package p\nimport a.b.C\n  import d.E as F\nval x = 1\n"
	if lazy.String() != eager.String() || lazy.NumSubexp() != eager.NumSubexp() {
		t.Fatal("metadata differs")
	}
	if lazy.MatchString(src) != eager.MatchString(src) || lazy.Match([]byte(src)) != eager.Match([]byte(src)) {
		t.Fatal("match differs")
	}
	if !slices.EqualFunc(lazy.FindAllStringSubmatch(src, -1), eager.FindAllStringSubmatch(src, -1), slices.Equal[[]string]) {
		t.Fatal("FindAllStringSubmatch differs")
	}
	if !slices.Equal(lazy.FindStringSubmatchIndex(src), eager.FindStringSubmatchIndex(src)) {
		t.Fatal("FindStringSubmatchIndex differs")
	}
	if lazy.ReplaceAllString(src, "<$1>") != eager.ReplaceAllString(src, "<$1>") {
		t.Fatal("ReplaceAllString differs")
	}
	first := lazy.Re()
	if again := lazy.Re(); again != first {
		t.Fatal("Re compiled twice")
	}
}

func TestRegexpCompilesOnceUnderConcurrency(t *testing.T) {
	r := New(`a+b`)
	var wg sync.WaitGroup
	got := make([]*regexp.Regexp, 32)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = r.Re()
		}(i)
	}
	wg.Wait()
	for _, re := range got {
		if re != got[0] {
			t.Fatal("concurrent callers saw different compiled regexps")
		}
	}
}

func TestInvalidPatternPanicsOnFirstUseAndFailsCompileAll(t *testing.T) {
	r := New(`(unclosed`)
	if err := CompileAll(); err == nil {
		t.Fatal("CompileAll accepted an invalid pattern")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on first use of an invalid pattern")
		}
		registryMu.Lock()
		registry = slices.DeleteFunc(registry, func(x *Regexp) bool { return x == r })
		registryMu.Unlock()
	}()
	r.MatchString("x")
}
