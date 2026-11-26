package rules_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runCTCAcrossFiles indexes decl and caller with one resolver and runs
// CompileTimeConstant on caller only.
func runCTCAcrossFiles(t *testing.T, decl, caller string) []scanner.Finding {
	t.Helper()
	declFile := parseInline(t, decl)
	callerFile := parseInline(t, caller)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{declFile, callerFile}, 1)
	for _, r := range api.Registry {
		if r.ID == "CompileTimeConstant" {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(callerFile)
			return cols.Findings()
		}
	}
	t.Fatal("CompileTimeConstant not registered")
	return nil
}

const ctcCrossFileDecl = `package lib

import com.google.errorprone.annotations.CompileTimeConstant

fun log(@CompileTimeConstant msg: String, extra: String = "") {}
fun plain(msg: String) {}
interface Sink { fun write(@CompileTimeConstant key: String) }
interface OtherSink { fun write(key: String) }
open class Base { open fun put(@CompileTimeConstant key: String) {} }
class Child : Base()
class Store {
    operator fun set(@CompileTimeConstant key: String, value: Int) {}
    operator fun get(@CompileTimeConstant key: String): Int = 0
}
class Acc { operator fun plusAssign(@CompileTimeConstant s: String) {} }
class Fn { operator fun invoke(@CompileTimeConstant s: String) {} }
object Logger { fun tag(@CompileTimeConstant t: String) {} }
interface Plain { fun take(k: String) }
`

// #1610: calls into other files, operator overloads and interface methods
// resolve to the annotated declaration through the type index.
func TestCompileTimeConstantAcrossFiles(t *testing.T) {
	caller := `package app

import lib.Acc
import lib.Base
import lib.Child
import lib.Fn
import lib.Logger
import lib.OtherSink
import lib.Sink
import lib.Store
import lib.log
import lib.plain

const val KEY = "k"

fun use(dynamic: String, sink: Sink, other: OtherSink, base: Base, child: Child, store: Store, acc: Acc, fn: Fn) {
    log(dynamic)
    log("ok")
    log(KEY, dynamic)
    plain(dynamic)
    sink.write(dynamic)
    other.write(dynamic)
    base.put(dynamic)
    child.put(dynamic)
    store[dynamic] = 1
    store["k"] = 1
    val x = store[dynamic]
    val y = store[KEY]
    acc += dynamic
    acc += "fine"
    fn(dynamic)
    fn("fine")
    Logger.tag(dynamic)
}
`
	want := ctcLinesOf(caller, "log(dynamic)", "sink.write(dynamic)", "base.put(dynamic)", "child.put(dynamic)",
		"store[dynamic] = 1", "val x = store[dynamic]", "acc += dynamic", "fn(dynamic)", "Logger.tag(dynamic)")
	findings := runCTCAcrossFiles(t, ctcCrossFileDecl, caller)
	var got []int
	for _, f := range findings {
		if !strings.Contains(f.Message, "Non-compile-time constant expression passed") {
			t.Fatalf("unexpected message %q", f.Message)
		}
		got = append(got, f.Line)
	}
	sort.Ints(got)
	if strconv.Quote(fmtInts(got)) != strconv.Quote(fmtInts(want)) {
		t.Fatalf("finding lines = %v, want %v (%v)", got, want, findings)
	}
}

// ctcLinesOf returns the 1-based line of each statement in src.
func ctcLinesOf(src string, statements ...string) []int {
	lines := strings.Split(src, "\n")
	var out []int
	for _, stmt := range statements {
		for i, l := range lines {
			if strings.TrimSpace(l) == stmt {
				out = append(out, i+1)
				break
			}
		}
	}
	return out
}

func fmtInts(v []int) string {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func TestCompileTimeConstantAcrossFilesNegatives(t *testing.T) {
	findings := runCTCAcrossFiles(t, ctcCrossFileDecl, `package app

import lib.Store
import lib.log

class Store2 { operator fun set(key: String, value: Int) {} }

class Screen {
    // A member shadows the imported top-level function.
    fun log(msg: String) {}

    fun use(dynamic: String, store: Store2, real: Store) {
        log(dynamic)
        store[dynamic] = 1
        real.toString()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

// Lint reports an override that adds the annotation to a supertype method
// declared in another file.
func TestCompileTimeConstantOverrideAcrossFiles(t *testing.T) {
	findings := runCTCAcrossFiles(t, ctcCrossFileDecl, `package app

import com.google.errorprone.annotations.CompileTimeConstant
import lib.Base
import lib.Plain
import lib.Sink

class AddsIt : Plain { override fun take(@CompileTimeConstant k: String) {} }
class KeepsIt : Sink { override fun write(@CompileTimeConstant key: String) {} }
class Sub : Base() { override fun put(@CompileTimeConstant key: String) {} }
`)
	if len(findings) != 1 || findings[0].Line != 8 || !strings.Contains(findings[0].Message, "cannot override method without it") {
		t.Fatalf("expected one override finding on line 8, got %v", findings)
	}
}
