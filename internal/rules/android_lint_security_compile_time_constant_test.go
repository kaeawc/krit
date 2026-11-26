package rules_test

import (
	"strings"
	"testing"
)

// Expected positions come from running Android Lint 32.4.1 on the same sources, with errorprone's
// @CompileTimeConstant on the classpath.

const compileTimeConstantKotlinSource = `package com.example

import com.google.errorprone.annotations.CompileTimeConstant

const val CONST = "c"
val topVal = "t"
var topVar = "v"

object Sql {
    fun query(@CompileTimeConstant sql: String) {}
    fun two(a: String, @CompileTimeConstant b: String) {}
    fun forward(@CompileTimeConstant sql: String) { query(sql) }
    fun list(@CompileTimeConstant items: List<String>) {}
}

class Holder(@CompileTimeConstant val ok: String) {
    @CompileTimeConstant private val field1 = "x"
    @CompileTimeConstant val publicField = "y"
    @CompileTimeConstant @JvmField val jvmField = "z"
    @CompileTimeConstant private var mutable = "m"
    @field:CompileTimeConstant private var mutable2 = "m"
}

class Caller {
    fun calls(param: String, flag: Boolean) {
        Sql.query("literal")
        Sql.query(CONST)
        Sql.query(topVal)
        Sql.query(topVar)
        Sql.query(param)
        Sql.query("a" + "b")
        Sql.query("a" + param)
        Sql.query("pre $CONST")
        Sql.query("pre $param")
        Sql.query(if (flag) "a" else "b")
        Sql.query(if (flag) "a" else param)
        val local = "l"
        Sql.query(local)
        var localVar = "lv"
        Sql.query(localVar)
        Sql.query(param.trim())
        Sql.two(param, "ok")
        Sql.two("ok", param)
        Sql.two(b = param, a = "x")
        Sql.list(listOf("a", "b"))
        Sql.list(listOf("a", param))
        Sql.query(String.format("%s", "x"))
        Sql.query(Constants.JAVA_CONST)
        Sql.query(Constants.javaNonFinal)
        val ref = Sql::query
    }
}
`

const compileTimeConstantJavaSource = `package com.example;

import com.google.errorprone.annotations.CompileTimeConstant;

public class Constants {
    public static final String JAVA_CONST = "j";
    public static String javaNonFinal = "n";
    @CompileTimeConstant public static String annotatedNonFinal = "a";
    @CompileTimeConstant public static final String annotatedFinal = "f";

    static void run(@CompileTimeConstant String sql) {}

    void calls(String p, final String q) {
        run("x");
        run(JAVA_CONST);
        run(p);
        run(javaNonFinal);
        final String loc = "loc";
        run(loc);
        String loc2 = "loc2";
        run(loc2);
        run("a" + JAVA_CONST);
        run(p + "x");
    }

    void forward(@CompileTimeConstant String sql) { run(sql); }
    void reassign(@CompileTimeConstant String sql) { sql = "x"; run(sql); }
}
`

const compileTimeConstantDeclarationsSource = `package com.example

import com.google.errorprone.annotations.CompileTimeConstant

class FieldParam(@field:CompileTimeConstant val a: String, @field:CompileTimeConstant private val b: String)

open class Base {
    open fun run(sql: String) {}
    open fun ok(@CompileTimeConstant sql: String) {}
}

class Child : Base() {
    override fun run(@CompileTimeConstant sql: String) {}
    override fun ok(@CompileTimeConstant sql: String) {}
}

class Setter {
    var prop: String = ""
        set(@CompileTimeConstant value) { field = value }
}

fun @receiver:CompileTimeConstant String.ext() {}

class Counter {
    fun bump(@CompileTimeConstant n: Int) {
        var x = n
        @CompileTimeConstant var y = 1
        y++
        --y
        y = 2
        y += 1
        y = n
        y = x
    }
}
`

func TestCompileTimeConstantLintParityKotlin(t *testing.T) {
	findings := runRuleByName(t, "CompileTimeConstant", compileTimeConstantKotlinSource)
	// Lint also reports Sql.query(Constants.javaNonFinal) at 49:19: the field is declared in another file, which this
	// port does not resolve (a known false negative).
	if got, want := findingPositions(findings), "18:5,20:5,21:5,28:19,29:19,30:19,32:19,34:19,36:19,40:19,41:19,43:23,44:21,46:18,47:19,50:19"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestCompileTimeConstantLintParityJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "CompileTimeConstant", compileTimeConstantJavaSource)
	if got, want := findingPositions(findings), "8:5,16:13,17:13,21:13,23:13,27:69"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestCompileTimeConstantLintParityDeclarations(t *testing.T) {
	findings := runRuleByName(t, "CompileTimeConstant", compileTimeConstantDeclarationsSource)
	if got, want := findingPositions(findings), "5:18,5:18,5:60,13:5,19:9,22:5,28:9,29:9,31:9,33:9"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
	messages := map[string]bool{}
	for _, f := range findings {
		messages[f.Message] = true
	}
	for _, want := range []string{
		"Annotating backing field of constructor property with `@field:CompileTimeConstant`\nis unsafe and unnecessary. Annotate as `@CompileTimeConstant` only.",
		"Method with `@CompileTimeConstant` parameter cannot override method without it",
		"Cannot apply increment or decrement operators to variable annotated with `@CompileTimeConstant`",
		"Non-compile-time constant expression assigned to variable annotated with `@CompileTimeConstant`",
	} {
		if !messages[want] {
			t.Errorf("missing message %q", want)
		}
	}
}

func TestCompileTimeConstantIgnoresUnresolvedAndUnannotated(t *testing.T) {
	src := `package com.example

import com.google.errorprone.annotations.CompileTimeConstant
import com.example.other.Sql

class Caller {
    fun execSql(sql: String) {}

    fun calls(param: String) {
        // Sql lives in another file: its parameters are not known here.
        Sql.query(param)
        execSql(param)
        Database.query(Config.QUERY)
    }
}

object Database {
    fun query(@CompileTimeConstant sql: String) {}
}
`
	if got := runRuleByName(t, "CompileTimeConstant", src); len(got) != 0 {
		t.Fatalf("reported calls it cannot resolve: %v", got)
	}
	if !strings.Contains(compileTimeConstantKotlinSource, "Constants.javaNonFinal") {
		t.Fatal("probe source changed")
	}
}

// TestCompileTimeConstantSkipsFilesWithoutTheAnnotation pins the file gate: a file is checked when it mentions
// @CompileTimeConstant, or (Kotlin) when it mentions a function declared elsewhere with an annotated parameter
// (#1610). A call in any other file is not checked, even when it would resolve to an annotated callee.
func TestCompileTimeConstantSkipsFilesWithoutTheAnnotation(t *testing.T) {
	decl := parseInline(t, "package com.example\n\nobject Sql {\n    fun query(@CompileTimeConstant sql: String) {}\n}\n")
	caller := parseInline(t, "package com.other\n\nimport com.example.Sql\n\nfun run(q: String) = Sql.query(q)\n")
	if got := runCTCProject(t, caller.Path, decl, caller); len(got) != 1 {
		t.Fatalf("expected the call through Sql.query to be checked, got %v", got)
	}
	// Without the declaring file in the index, nothing names an annotated function: the caller is not checked.
	if got := runCTCProject(t, caller.Path, caller); len(got) != 0 {
		t.Fatalf("expected no findings without the annotated declaration, got %v", got)
	}
}
