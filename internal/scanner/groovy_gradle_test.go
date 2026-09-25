package scanner

import (
	"context"
	"strings"
	"testing"
)

func TestGroovyGradleParseAndSuppression(t *testing.T) {
	f := ParseGradleScript(context.Background(), "build.gradle", []byte("// krit:ignore[MagicNumber]\nfoo 42\n"), nil)
	if f.FlatTree == nil || !f.IsGroovyGradle() {
		t.Fatal("expected parsed Groovy Gradle file")
	}
	sf := BuildSuppressionFilter(f, nil, nil, "")
	if !sf.IsSuppressed("MagicNumber", "", 1) {
		t.Fatal("inline suppression was not retained")
	}
	k := ParseGradleScript(context.Background(), "build.gradle.kts", []byte("plugins {}"), nil)
	if k.FlatTree == nil || k.IsGroovyGradle() {
		t.Fatal("expected parsed Kotlin Gradle file")
	}
}

func TestGroovyHelpersShapes(t *testing.T) {
	tests := []struct{ name, src string }{
		{"plugins", `plugins { id 'com.android.application'
 id "org.jetbrains.kotlin.android" version "1.9.0" apply false
 alias(libs.plugins.hilt) }`},
		{"dependencies", `dependencies {
 implementation 'androidx.core:core-ktx:1.12.0'
 implementation "org.jetbrains.kotlin:kotlin-stdlib:$kotlin_version"
 implementation group: 'com.squareup.okhttp3', name: 'okhttp', version: '4.12.0'
 kapt("com.google.dagger:hilt-compiler:2.50") {
 exclude group: 'org.jetbrains'
 }
 // implementation 'commented:out:1.0'
}`},
		{"android", `android {
 compileSdk 34
 buildFeatures { compose true; viewBinding = true }
}`},
		{"tasks", `tasks.register('hello') {
 doLast { println "hi" }
}`},
		{"next line closure", "foo('x')\n{ bar() }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := ParseGradleScript(context.Background(), "x.gradle", []byte(tt.src), nil)
			if f.FlatTree == nil {
				t.Fatal("nil tree")
			}
			calls := make(map[string][]uint32)
			for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
				n := groovyNodeType(f.FlatTree, i)
				if n == "function_call" || n == "juxt_function_call" {
					calls[GroovyCallName(f.FlatTree, i, f.Content)] = append(calls[GroovyCallName(f.FlatTree, i, f.Content)], i)
				}
			}
			switch tt.name {
			case "plugins":
				chain := GroovyCommandChain(f.FlatTree, calls["id"][1], f.Content)
				if len(chain) != 3 {
					t.Fatalf("id chain len=%d", len(chain))
				}
				for i, want := range []string{"id", "version", "apply"} {
					if got := GroovyCallName(f.FlatTree, chain[i], f.Content); got != want {
						t.Errorf("chain[%d]=%q", i, got)
					}
				}
				if groovyNodeType(f.FlatTree, calls["alias"][0]) != "function_call" {
					t.Error("alias not function_call")
				}
			case "dependencies":
				lit, ok := groovyFindDescendant(f.FlatTree, calls["implementation"][0], "string")
				if !ok {
					t.Fatal("missing dependency string")
				}
				if got, ok := GroovyStringLiteral(f.FlatTree, lit, f.Content); !ok || got != "androidx.core:core-ktx:1.12.0" {
					t.Errorf("literal=%q,%v", got, ok)
				}
				interp, _ := groovyFindDescendant(f.FlatTree, calls["implementation"][1], "string")
				if _, ok := GroovyStringLiteral(f.FlatTree, interp, f.Content); ok {
					t.Error("interpolated string accepted")
				}
				if _, ok := GroovyCallClosure(f.FlatTree, calls["kapt"][0], f.Content); !ok {
					t.Error("kapt closure missing")
				}
				for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
					if groovyNodeType(f.FlatTree, i) == "comment" && strings.Contains(FlatNodeText(f.FlatTree, i, f.Content), "implementation") {
						if len(calls["implementation"]) != 3 {
							t.Errorf("comment generated call count=%d", len(calls["implementation"]))
						}
					}
				}
			case "android":
				if _, ok := GroovyCallClosure(f.FlatTree, calls["android"][0], f.Content); !ok {
					t.Error("android closure missing")
				}
				if got := GroovyCommandChain(f.FlatTree, calls["compose"][0], f.Content); len(got) != 1 {
					t.Errorf("compose chained across semicolon: %v", got)
				}
			case "tasks":
				i := calls["tasks.register"][0]
				if _, ok := GroovyCallClosure(f.FlatTree, i, f.Content); !ok {
					t.Error("tasks.register closure missing")
				}
			case "next line closure":
				if _, ok := GroovyCallClosure(f.FlatTree, calls["foo"][0], f.Content); ok {
					t.Error("attached next-line closure")
				}
			}
		})
	}
	t.Run("multiline call closure", func(t *testing.T) {
		src := `dependencies {
 implementation("a:b:1",
     "c:d:2") {
   transitive = false
 }
}`
		f := ParseGradleScript(context.Background(), "x.gradle", []byte(src), nil)
		var call uint32
		for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
			if groovyNodeType(f.FlatTree, i) == "function_call" && GroovyCallName(f.FlatTree, i, f.Content) == "implementation" {
				call = i
				break
			}
		}
		if call == 0 {
			t.Fatal("implementation call missing")
		}
		if _, ok := GroovyCallClosure(f.FlatTree, call, f.Content); !ok {
			t.Fatal("multiline implementation closure missing")
		}
	})
	t.Run("semicolon command calls", func(t *testing.T) {
		src := `buildFeatures { compose true; viewBinding true }`
		f := ParseGradleScript(context.Background(), "x.gradle", []byte(src), nil)
		var compose, viewBinding uint32
		for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
			if groovyNodeType(f.FlatTree, i) != "juxt_function_call" {
				continue
			}
			switch GroovyCallName(f.FlatTree, i, f.Content) {
			case "compose":
				compose = i
			case "viewBinding":
				viewBinding = i
			}
		}
		if compose == 0 || viewBinding == 0 {
			t.Fatal("compose/viewBinding did not parse as juxt_function_call siblings")
		}
		if got := GroovyCommandChain(f.FlatTree, compose, f.Content); len(got) != 1 {
			t.Fatalf("compose chained across semicolon: %v", got)
		}
	})
	t.Run("newline command calls", func(t *testing.T) {
		src := "buildFeatures {\n compose true\n viewBinding true\n}"
		f := ParseGradleScript(context.Background(), "x.gradle", []byte(src), nil)
		var compose uint32
		for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
			if groovyNodeType(f.FlatTree, i) == "juxt_function_call" && GroovyCallName(f.FlatTree, i, f.Content) == "compose" {
				compose = i
				break
			}
		}
		if compose == 0 {
			t.Fatal("compose juxt call missing")
		}
		if got := GroovyCommandChain(f.FlatTree, compose, f.Content); len(got) != 1 {
			t.Fatalf("compose chained across newline: %v", got)
		}
	})
	t.Run("string literal edge cases", func(t *testing.T) {
		src := `a = 'it\'s'; b = '$notInterpolated'; c = /slashy/; d = $/dollar slashy/$`
		f := ParseGradleScript(context.Background(), "x.gradle", []byte(src), nil)
		var escaped, dollar, slashy, dollarSlashy uint32
		for i := uint32(1); i < uint32(f.FlatTree.Len()); i++ {
			if groovyNodeType(f.FlatTree, i) == "string" {
				switch FlatNodeText(f.FlatTree, i, f.Content) {
				case `'it\'s'`:
					escaped = i
				case `'$notInterpolated'`:
					dollar = i
				}
			}
			text := FlatNodeText(f.FlatTree, i, f.Content)
			if text == "/slashy/" {
				slashy = i
			}
			if text == "$/dollar slashy/$" {
				dollarSlashy = i
			}
		}
		if escaped == 0 || dollar == 0 {
			t.Fatal("quoted string cases missing from parse tree")
		}
		if _, ok := GroovyStringLiteral(f.FlatTree, escaped, f.Content); ok {
			t.Error("escaped quote accepted")
		}
		if got, ok := GroovyStringLiteral(f.FlatTree, dollar, f.Content); !ok || got != "$notInterpolated" {
			t.Errorf("single quoted dollar=%q,%v", got, ok)
		}
		if slashy == 0 || dollarSlashy == 0 {
			t.Fatal("slashy string forms missing from parse tree")
		}
		if groovyNodeType(f.FlatTree, slashy) != "string" || groovyNodeType(f.FlatTree, dollarSlashy) != "string" {
			t.Log("slashy forms use a distinct grammar node type")
		}
		if _, ok := GroovyStringLiteral(f.FlatTree, slashy, f.Content); ok {
			t.Error("slashy string accepted")
		}
		if _, ok := GroovyStringLiteral(f.FlatTree, dollarSlashy, f.Content); ok {
			t.Error("dollar-slashy string accepted")
		}
	})
}
