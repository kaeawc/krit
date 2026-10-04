package javafacts

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSourceFactsForFile_ExtractsJavaSemanticFacts(t *testing.T) {
	file := parseJavaSource(t, "Example.java", `
package com.example;

import static java.util.Collections.emptyList;
import android.webkit.WebView;
import java.util.*;

@Deprecated
class Example extends Base implements Runnable, Closeable {
  private final List<String> names = emptyList();

  @Override
  public String value(WebView webView) {
    return "";
  }

  static class Nested implements Runnable {
    @Override public void run() {}
  }
}
`)
	facts := SourceFactsForFile(file)
	if facts.Package != "com.example" {
		t.Fatalf("Package = %q, want com.example", facts.Package)
	}
	if got := facts.Imports["WebView"]; got != "android.webkit.WebView" {
		t.Fatalf("WebView import = %q", got)
	}
	if got := facts.StaticImports["emptyList"]; got != "java.util.Collections.emptyList" {
		t.Fatalf("static import = %q", got)
	}
	if !containsString(facts.WildcardImports, "java.util") {
		t.Fatalf("WildcardImports = %#v, want java.util", facts.WildcardImports)
	}
	example := facts.Classes["Example"]
	if example.FQN != "com.example.Example" {
		t.Fatalf("Example FQN = %q", example.FQN)
	}
	for _, want := range []string{"Base", "Runnable", "Closeable"} {
		if !containsString(example.Supertypes, want) {
			t.Fatalf("Example supertypes missing %q: %#v", want, example.Supertypes)
		}
	}
	if !containsString(example.Annotations, "Deprecated") {
		t.Fatalf("Example annotations = %#v", example.Annotations)
	}
	if containsString(example.Annotations, "Override") {
		t.Fatalf("class annotations should not include method annotations: %#v", example.Annotations)
	}
	if got := example.Fields["names"].Type; got != "List" {
		t.Fatalf("field type = %q, want List", got)
	}
	methods := example.Methods["value"]
	if len(methods) != 1 || methods[0].ReturnType != "String" || !containsString(methods[0].Annotations, "Override") {
		t.Fatalf("method facts = %#v", methods)
	}
	nested := facts.Classes["Nested"]
	if nested.FQN != "com.example.Example.Nested" {
		t.Fatalf("Nested FQN = %q", nested.FQN)
	}
}

func TestJavaHeaderSupertypes_GenericArguments(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   []string
	}{
		{"type argument is not a supertype", "class Foo implements TriFunc<String, X509TrustManager, String> {}", []string{"TriFunc"}},
		{"nested type arguments", "class Foo extends Base implements Map<String, List<X509TrustManager>> {}", []string{"Base", "Map"}},
		{"mixed supertypes", "class Foo implements A, B<C, D>, E {}", []string{"A", "B", "E"}},
		{"plain supertypes unchanged", "class Foo extends Base implements Runnable, Closeable {}", []string{"Base", "Runnable", "Closeable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := javaHeaderSupertypes(tc.header); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("javaHeaderSupertypes(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

func TestJavaHeaderSupertypes_AnnotatedTypeArgument(t *testing.T) {
	file := parseJavaSource(t, "Foo.java", `
package test;
@interface Marker { String value(); }
interface TriFunc<A, B, C> {}
class Foo implements TriFunc<@Marker(value = "a,b") String, X509TrustManager, String> {}
`)
	if file.FlatTree.Node(0).HasError() {
		t.Fatal("tree-sitter rejected annotated type argument")
	}
	var header string
	file.FlatWalkNodes(0, "class_declaration", func(node uint32) {
		header = javaDeclarationHeader(file.FlatNodeText(node))
	})
	if !strings.Contains(header, `@Marker(value = "a,b") String`) {
		t.Fatalf("annotated type argument absent from Java declaration header: %q", header)
	}
	if got, want := javaHeaderSupertypes(header), []string{"TriFunc"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("javaHeaderSupertypes(%q) = %v, want %v", header, got, want)
	}
}

func TestSourceFactsResolveType_ImportsWildcardsPackageLocalAndKnownTypes(t *testing.T) {
	local := parseJavaSource(t, "LocalRoom.java", `
package com.example;

class Room {}
`)
	use := parseJavaSource(t, "UseTypes.java", `
package com.example;

import java.util.*;
import android.webkit.WebView;

class UseTypes {
  WebView webView;
  List<String> names;
  Room room;
  String label;
}
`)
	index := SourceIndexForFiles([]*scanner.File{local, use})
	facts := SourceFactsForFile(use)
	for name, want := range map[string]string{
		"WebView": "android.webkit.WebView",
		"List":    "java.util.List",
		"Room":    "com.example.Room",
		"String":  "java.lang.String",
	} {
		if got := facts.ResolveType(name, index); got != want {
			t.Fatalf("ResolveType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSourceFactsImportsOrMentions_DistinguishesJavaLocalLookalikes(t *testing.T) {
	local := parseJavaSource(t, "Browser.java", `
package test;

class WebView {
  void addJavascriptInterface(Object bridge, String name) {}
}

class Browser {
  void setup(WebView webView) {
    webView.addJavascriptInterface(new Object(), "bridge");
  }
}
`)
	real := parseJavaSource(t, "RealBrowser.java", `
package test;

import android.webkit.WebView;

class RealBrowser {
  void setup(WebView webView) {
    webView.addJavascriptInterface(new Object(), "bridge");
  }
}
`)
	if SourceFactsForFile(local).ImportsOrMentions("android.webkit.WebView") {
		t.Fatal("local WebView lookalike should not resolve as android.webkit.WebView")
	}
	if !SourceFactsForFile(real).ImportsOrMentions("android.webkit.WebView") {
		t.Fatal("imported WebView should resolve as android.webkit.WebView")
	}
}

// TestSourceFactsImportsOrMentions_IgnoresFqnsInsideCommentsAndStrings is
// a lexical-negative regression for the ImportsOrMentions fallback. The
// previous implementation fell back to `strings.Contains(content, fqn)`,
// so a Javadoc or string literal that happened to contain the FQN would
// wrongly count as a real reference. After the AST-aware rewrite, the
// haystack is built from scoped-identifier nodes only and comments /
// string literals are excluded by construction.
func TestSourceFactsImportsOrMentions_IgnoresFqnsInsideCommentsAndStrings(t *testing.T) {
	file := parseJavaSource(t, "CommentsOnly.java", `
package test;

/**
 * Historical note: this class used to depend on android.webkit.WebView,
 * but the integration has since been removed.
 */
class CommentsOnly {
  // android.webkit.WebView used to be the bridge target.
  private static final String NOTE = "android.webkit.WebView is no longer used";

  void run() {
    /* android.webkit.WebView -- still referenced in CHANGELOG */
    System.out.println(NOTE);
  }
}
`)
	if SourceFactsForFile(file).ImportsOrMentions("android.webkit.WebView") {
		t.Fatal("FQN appears only inside comments and string literals — ImportsOrMentions must return false")
	}
}

// TestSourceFactsImportsOrMentions_FindsFullyQualifiedReferenceInCode
// pins down the positive side of the lexical-aware rewrite: when the
// FQN really is written out in code (a fully qualified reference with
// no import), ImportsOrMentions must still report true so that rules
// guarded by it do not regress.
func TestSourceFactsImportsOrMentions_FindsFullyQualifiedReferenceInCode(t *testing.T) {
	file := parseJavaSource(t, "FullyQualified.java", `
package test;

class FullyQualified {
  void setup(android.webkit.WebView webView) {
    webView.addJavascriptInterface(new Object(), "bridge");
  }
}
`)
	if !SourceFactsForFile(file).ImportsOrMentions("android.webkit.WebView") {
		t.Fatal("fully qualified WebView reference should be detected")
	}
}

func TestSourceFactsResolveType_KnowsJavaLangSystemRuntimeAndLocalShadows(t *testing.T) {
	file := parseJavaSource(t, "Lifecycle.java", `
package test;

class Lifecycle {
  void shutdown() {
    System.exit(1);
    Runtime.getRuntime().gc();
  }
}
`)
	facts := SourceFactsForFile(file)
	if got := facts.ResolveType("System", nil); got != "java.lang.System" {
		t.Fatalf("ResolveType(System) = %q, want java.lang.System", got)
	}
	if got := facts.ResolveType("Runtime", nil); got != "java.lang.Runtime" {
		t.Fatalf("ResolveType(Runtime) = %q, want java.lang.Runtime", got)
	}

	shadowed := parseJavaSource(t, "Shadowed.java", `
package test;

class System {}
class Runtime {}
`)
	shadowFacts := SourceFactsForFile(shadowed)
	if got := shadowFacts.ResolveType("System", nil); got != "test.System" {
		t.Fatalf("ResolveType(System) with local shadow = %q, want test.System", got)
	}
	if got := shadowFacts.ResolveType("Runtime", nil); got != "test.Runtime" {
		t.Fatalf("ResolveType(Runtime) with local shadow = %q, want test.Runtime", got)
	}
}

func parseJavaSource(t *testing.T, name, src string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
