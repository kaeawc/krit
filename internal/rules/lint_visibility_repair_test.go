package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runVisibilityRepairRule(t *testing.T, language, name, source string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test."+language)
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	parse := scanner.ParseFile
	if language == "java" {
		parse = scanner.ParseJavaFile
	}
	file, err := parse(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range api.Registry {
		if rule.ID == name {
			columns := NewDispatcher([]*api.Rule{rule}, nil).Run(file)
			return columns.Findings()
		}
	}
	t.Fatalf("rule %q not found", name)
	return nil
}

func TestXmlExternalEntity_LogTagMismatch_KotlinVisibilityRepair(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"superclass", "open class Base { class LOOKALIKE }; class Loader : Base() { USE }", 0},
		{"transitive superclass", "open class Base { class LOOKALIKE }; open class Middle : Base(); class Loader : Middle() { USE }", 0},
		{"interface supertype", "interface Base { class LOOKALIKE }; class Loader : Base { USE }", 0},
		{"enum superclass", "open class Base { class LOOKALIKE }; enum class Loader : Base() { A; USE }", 0},
		{"object superclass", "open class Base { class LOOKALIKE }; object Loader : Base() { USE }", 0},
		{"companion superclass", "open class Base { class LOOKALIKE }; class Loader { companion object : Base() { USE } }", 0},
		{"object literal superclass", "open class Base { class LOOKALIKE }; val loader = object : Base() { USE }", 0},
		{"outer superclass", "open class Base { class LOOKALIKE }; class Loader : Base() { class Inner { USE } }", 0},
		{"outer object superclass", "open class Base { class LOOKALIKE }; object Loader : Base() { class Inner { USE } }", 0},
		{"outer companion superclass", "open class Base { class LOOKALIKE }; class Loader { companion object : Base() { class Inner { USE } } }", 0},
		{"outer object literal superclass", "open class Base { class LOOKALIKE }; val loader = object : Base() { val inner = object { USE } }", 0},
		{"interface enclosing use", "interface Base { class LOOKALIKE }; interface Loader : Base { USE }", 0},
		{"nested object lookalike", "class Loader { object LOOKALIKE; USE }", 0},
		{"inherited nested object lookalike", "open class Base { object LOOKALIKE }; class Loader : Base() { USE }", 0},
		{"nested enum lookalike", "class Loader { enum class LOOKALIKE { A }; USE }", 0},
		{"nested interface lookalike", "class Loader { interface LOOKALIKE; USE }", 0},
		{"typealias lookalike", "typealias LOOKALIKE = Custom; class Custom; USE", 0},
		{"companion lookalike", "class Loader { companion object { class LOOKALIKE }; USE }", 0},
		{"inherited companion lookalike", "open class Base { companion object { class LOOKALIKE } }; class Loader : Base() { USE }", 0},
		{"enum companion lookalike", "enum class Loader { A; companion object { class LOOKALIKE }; USE }", 0},
		{"unrelated nested class", "class Other { class LOOKALIKE }; USE", 1},
		{"unrelated nested object", "class Other { object LOOKALIKE }; USE", 1},
		{"unrelated enum body", "enum class Other { A; class LOOKALIKE }; USE", 1},
		{"unrelated interface body", "interface Other { class LOOKALIKE }; USE", 1},
		{"within enum body", "enum class Loader { A; class LOOKALIKE; USE }", 0},
		{"within interface body", "interface Loader { class LOOKALIKE; USE }", 0},
		{"other function local", "fun other() { class LOOKALIKE }; USE", 1},
		{"other lambda local", "val other = { class LOOKALIKE }; USE", 1},
		{"local in use scope", "fun enclosing() { class LOOKALIKE; USE }", 0},
		{"local in use lambda", "val enclosing = { class LOOKALIKE; USE }", 0},
		{"forward reference retained", "fun enclosing() { USE; class LOOKALIKE }", 0},
		{"qualified supername retained", "open class Base { class LOOKALIKE }; class Loader : com.lib.Base() { USE }", 0},
		{"imported supername retained", "import com.lib.Base\nopen class Base { class LOOKALIKE }; class Loader : Base() { USE }", 0},
		{"ambiguous supername suppresses", "class Other { open class Base }; open class Base { class LOOKALIKE }; class Loader : Base() { USE }", 0},
		{"cycle with lookalike", "open class Base : Middle() { class LOOKALIKE }; open class Middle : Base(); class Loader : Middle() { USE }", 0},
		{"cycle without lookalike", "open class Base : Middle(); open class Middle : Base(); class Loader : Base() { USE }", 1},
		{"self cycle", "open class Base : Base(); class Loader : Base() { USE }", 1},
	}
	for _, rule := range []struct {
		name, lookalike, use string
	}{
		{"XmlExternalEntity", "DocumentBuilderFactory", "fun use(input: java.io.InputStream) = DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(input)"},
		{"LogTagMismatch", "Log", `fun use() { if (Log.isLoggable("a", Log.DEBUG)) { Log.i("b", "m") } }`},
	} {
		for _, test := range cases {
			t.Run(rule.name+"/"+test.name, func(t *testing.T) {
				source := strings.NewReplacer("LOOKALIKE", rule.lookalike, "USE", rule.use).Replace(test.src)
				source = "package test\nimport javax.xml.parsers.DocumentBuilderFactory\nimport android.util.Log\n" + source
				findings := runVisibilityRepairRule(t, "kt", rule.name, source)
				want := test.want
				if rule.name == "LogTagMismatch" {
					want *= 2
				}
				if len(findings) != want {
					t.Fatalf("want %d findings, got %v", want, findings)
				}
			})
		}
	}
}

func TestXmlExternalEntity_LogTagMismatch_JavaVisibilityRepair(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"superclass", "class Base { static class LOOKALIKE {} } class Loader extends Base { USE }", 0},
		{"transitive superclass", "class Base { static class LOOKALIKE {} } class Middle extends Base {} class Loader extends Middle { USE }", 0},
		{"generic superclass", "class Base<T> { static class LOOKALIKE {} } class Loader extends Base<String> { USE }", 0},
		{"interface supertype", "interface Base { class LOOKALIKE {} } class Loader implements Base { USE }", 0},
		{"multiple interface supertypes", "interface Other {} interface Base { class LOOKALIKE {} } class Loader implements Other, Base { USE }", 0},
		{"transitive interface", "interface Base { class LOOKALIKE {} } interface Middle extends Base {} class Loader implements Middle { USE }", 0},
		{"generic interface", "interface Base<T> { class LOOKALIKE {} } class Loader implements Base<String> { USE }", 0},
		{"interface enclosing use", "interface Base { class LOOKALIKE {} } interface Loader extends Base { USE }", 0},
		{"enum enclosing use", "interface Base { class LOOKALIKE {} } enum Loader implements Base { A; USE }", 0},
		{"record enclosing use", "interface Base { class LOOKALIKE {} } record Loader() implements Base { USE }", 0},
		{"anonymous class", "class Base { static class LOOKALIKE {} } class Loader { Object loader = new Base() { USE }; }", 0},
		{"anonymous interface", "interface Base { class LOOKALIKE {} } class Loader { Object loader = new Base() { USE }; }", 0},
		{"outer superclass", "class Base { static class LOOKALIKE {} } class Loader extends Base { class Inner { USE } }", 0},
		{"outer interface supertype", "interface Base { class LOOKALIKE {} } interface Loader extends Base { class Inner { USE } }", 0},
		{"outer anonymous superclass", "class Base { static class LOOKALIKE {} } class Loader { Object loader = new Base() { class Inner { USE } }; }", 0},
		{"outer enum interface", "interface Base { class LOOKALIKE {} } enum Loader implements Base { A; class Inner { USE } }", 0},
		{"outer record interface", "interface Base { class LOOKALIKE {} } record Loader() implements Base { class Inner { USE } }", 0},
		{"nested interface lookalike", "class Loader { interface LOOKALIKE {} USE }", 0},
		{"nested enum lookalike", "class Loader { enum LOOKALIKE { A } USE }", 0},
		{"nested record lookalike", "class Loader { record LOOKALIKE() {} USE }", 0},
		{"unrelated nested class", "class Other { static class LOOKALIKE {} } class Loader { USE }", 1},
		{"unrelated interface body", "interface Other { class LOOKALIKE {} } class Loader { USE }", 1},
		{"unrelated enum body", "enum Other { A; static class LOOKALIKE {} } class Loader { USE }", 1},
		{"within interface body", "interface Loader { class LOOKALIKE {} USE }", 0},
		{"within enum body", "enum Loader { A; static class LOOKALIKE {} USE }", 0},
		{"enum constant body", "interface Base { class LOOKALIKE {} } enum Loader implements Base { A { USE }; }", 0},
		{"anonymous body lookalike", "class Loader { Object loader = new Object() { class LOOKALIKE {} USE }; }", 0},
		{"unrelated anonymous body", "class Loader { Object other = new Object() { class LOOKALIKE {} }; USE }", 1},
		{"other method local", "class Loader { void other() { class LOOKALIKE {} } USE }", 1},
		{"constructor local", "class Loader { Loader() { class LOOKALIKE {} } USE }", 1},
		{"other block local", "class Loader { void enclosing() { { class LOOKALIKE {} } STATEMENT } }", 1},
		{"other lambda local", "class Loader { Runnable other = () -> { class LOOKALIKE {} }; USE }", 1},
		{"local in use block", "class Loader { void enclosing() { class LOOKALIKE {} STATEMENT } }", 0},
		{"local in use constructor", "class Loader { Loader() { class LOOKALIKE {} STATEMENT } }", 0},
		{"local in use lambda", "class Loader { Runnable enclosing = () -> { class LOOKALIKE {} STATEMENT }; }", 0},
		{"forward reference retained", "class Loader { void enclosing() { STATEMENT class LOOKALIKE {} } }", 0},
		{"qualified supername retained", "class Base { static class LOOKALIKE {} } class Loader extends com.lib.Base { USE }", 0},
		{"cycle with lookalike", "class Base extends Middle { static class LOOKALIKE {} } class Middle extends Base {} class Loader extends Middle { USE }", 0},
		{"cycle without lookalike", "class Base extends Middle {} class Middle extends Base {} class Loader extends Base { USE }", 1},
		{"self cycle", "class Base extends Base {} class Loader extends Base { USE }", 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			statement := "DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(input);"
			use := "void use(java.io.InputStream input) throws Exception { " + statement + " }"
			source := "import javax.xml.parsers.DocumentBuilderFactory;\n" + strings.NewReplacer("LOOKALIKE", "DocumentBuilderFactory", "USE", use, "STATEMENT", statement).Replace(test.src)
			findings := runVisibilityRepairRule(t, "java", "XmlExternalEntity", source)
			if len(findings) != test.want {
				t.Fatalf("want %d findings, got %v", test.want, findings)
			}

			logSource := strings.ReplaceAll(source, "DocumentBuilderFactory", "Log")
			logSource = strings.ReplaceAll(logSource, "Log.newInstance().newDocumentBuilder().parse(input);", `if (Log.isLoggable("a", Log.DEBUG)) { Log.i("b", "m"); }`)
			path := filepath.Join(t.TempDir(), "test.java")
			if err := os.WriteFile(path, []byte(logSource), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := scanner.ParseJavaFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			checked := false
			file.FlatWalkNodes(0, "method_invocation", func(node uint32) {
				call, ok := lintCallAt(file, node)
				if ok && call.Name == "isLoggable" {
					checked = true
					if visible := fileDeclaresTypeVisibleAt(file, "Log", node); visible != (test.want == 0) {
						t.Errorf("Log visibility = %v, want %v", visible, test.want == 0)
					}
				}
			})
			if !checked {
				t.Fatal("missing Java use site")
			}
		})
	}
}

func TestObjectExtendsThrowable_EnclosingVisibilityRepair(t *testing.T) {
	for _, source := range []string{
		"open class Base { open class Exception }; object Loader : Base() { object Failure : Exception() }",
		"open class Base { open class Exception }; class Loader { companion object : Base() { object Failure : Exception() } }",
		"open class Base { open class Exception }; val loader = object : Base() { val failure = object : Exception() {} }",
		"open class Base { open class Exception }; class Loader : Base() { class Inner { object Failure : Exception() } }",
		"open class Base { open class Exception }; open class Middle : Base(); class Loader : Middle() { object Failure : Exception() }",
		"open class Base : Middle() { open class Exception }; open class Middle : Base(); class Loader : Middle() { object Failure : Exception() }",
		"typealias Exception = Custom; open class Custom; object Failure : Exception()",
	} {
		t.Run(source, func(t *testing.T) {
			if findings := runVisibilityRepairRule(t, "kt", "ObjectExtendsThrowable", "package test\n"+source); len(findings) != 0 {
				t.Fatalf("want no findings, got %v", findings)
			}
		})
	}
}

func TestHandlerLeak_NullWrappersAndBranchesRepair(t *testing.T) {
	for _, test := range []struct {
		lang, expr string
		want       int
	}{
		{"kt", "requireNotNull(Looper.myLooper())", 0},
		{"kt", `requireNotNull(Looper.myLooper()) { "missing" }`, 0},
		{"kt", `checkNotNull(Looper.myLooper()) { "missing" }`, 0},
		{"kt", "checkNotNull(callback)", 1},
		{"kt", `checkNotNull(callback) { "missing" }`, 1},
		{"kt", "requireNotNull(callback)", 1},
		{"kt", "callbackOrNull ?: otherCallback", 1},
		{"java", "Objects.requireNonNull(Looper.myLooper())", 0},
		{"java", "java.util.Objects.requireNonNull(Looper.myLooper())", 0},
		{"java", "requireNonNull(Looper.myLooper())", 0},
		{"java", "l != null ? l : Looper.getMainLooper()", 0},
		{"java", "l == null ? Looper.getMainLooper() : l", 0},
		{"java", "Objects.requireNonNull(l != null ? l : Looper.getMainLooper())", 0},
		{"java", "Objects.requireNonNull(callback)", 1},
		{"java", "requireNonNull(callback)", 1},
		{"java", "callbackOrNull != null ? callbackOrNull : otherCallback", 1},
		{"java", "Looper.myLooper() != null ? callback : otherCallback", 1},
	} {
		t.Run(test.lang+"/"+test.expr, func(t *testing.T) {
			var findings []scanner.Finding
			if test.lang == "java" {
				source := "import android.os.Handler; import android.os.Looper; import java.util.Objects; import static java.util.Objects.requireNonNull; class Outer { Handler.Callback callback, callbackOrNull, otherCallback; Looper l; Handler handler = new Handler(" + test.expr + ") {}; }"
				findings = runVisibilityRepairRule(t, "java", "HandlerLeak", source)
			} else {
				source := "import android.os.Handler\nimport android.os.Looper\nclass Outer(val callback: Handler.Callback, val callbackOrNull: Handler.Callback?, val otherCallback: Handler.Callback) { val handler = object : Handler(" + test.expr + ") {} }"
				findings = runVisibilityRepairRule(t, "kt", "HandlerLeak", source)
			}
			if len(findings) != test.want {
				t.Fatalf("want %d findings, got %v", test.want, findings)
			}
		})
	}
}
