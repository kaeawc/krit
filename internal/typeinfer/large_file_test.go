package typeinfer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// Regression: rules_kotlin's examples/multiplex/src/Big.kt (2,000
// top-level classes, ~40k Int parameters) hung indexing for over ten
// minutes. Every stdlib-named parameter type re-resolved the file
// package by rescanning all top-level children with an index loop that
// was itself quadratic.
func TestIndexManyTopLevelDeclarationsCompletes(t *testing.T) {
	var b strings.Builder
	b.WriteString("package com.foo.bar\n\n")
	const classes = 3000
	for i := 1; i <= classes; i++ {
		fmt.Fprintf(&b, "class A%d {\n", i)
		for j := 1; j <= 10; j++ {
			fmt.Fprintf(&b, "    fun a%d(a: Int, b: Int) = a + b\n", j)
		}
		b.WriteString("}\n\n")
	}
	file := parseTestFile(t, b.String())

	r := NewResolver()
	done := make(chan struct{})
	go func() {
		r.IndexFilesParallel([]*scanner.File{file}, 1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("indexing a file with many top-level declarations did not finish in 30s")
	}
	if r.ClassHierarchy(fmt.Sprintf("com.foo.bar.A%d", classes)) == nil {
		t.Fatalf("last class was not indexed under the file package")
	}
}

func TestScanFileHeadersFlatSkipsDeclarations(t *testing.T) {
	file := parseTestFile(t, `@file:JvmName("Names")
package a.b

import c.D
import e.F as G
import h.*

class E
fun f() = Unit
`)
	headers := scanFileHeadersFlat(file)
	if headers.pkg != "a.b" {
		t.Errorf("pkg = %q, want a.b", headers.pkg)
	}
	if got := headers.it.Explicit["D"]; got != "c.D" {
		t.Errorf("Explicit[D] = %q, want c.D", got)
	}
	if got := headers.it.Aliases["G"]; got != "e.F" {
		t.Errorf("Aliases[G] = %q, want e.F", got)
	}
	if len(headers.it.Wildcard) != 1 || headers.it.Wildcard[0] != "h" {
		t.Errorf("Wildcard = %v, want [h]", headers.it.Wildcard)
	}
}
