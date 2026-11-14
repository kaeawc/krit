package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestCallReturnType_VisibilityAndAmbiguity checks which same-name function
// declarations a call's return type may come from. want is "" when the
// resolver must decline (unknown type), otherwise the rendered return type.
func TestCallReturnType_VisibilityAndAmbiguity(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		call  string // call_expression text in the caller file
		want  string
	}{
		{
			name: "private non-null overload in another file is invisible (indexed last)",
			files: map[string]string{
				"b_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nprivate fun maker(x: Int): String = \"a\"\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "private non-null overload in another file is invisible (indexed first)",
			files: map[string]string{
				"a_other.kt":  "package p\nprivate fun maker(x: Int): String = \"a\"\n",
				"b_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "private member of a class in another file is invisible",
			files: map[string]string{
				"b_caller.kt": "package p\nclass Caller {\n  fun maker(x: String): String? = null\n  fun use(x: String) = maker(x)\n}\n",
				"z_other.kt":  "package p\nclass Holder {\n  private fun maker(x: Int): String = \"a\"\n}\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "public member of a private class in another file is invisible",
			files: map[string]string{
				"b_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nprivate class Holder {\n  fun maker(x: Int): String = \"a\"\n}\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "local function in another file is invisible",
			files: map[string]string{
				"b_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nfun outer(): String {\n  fun maker(x: Int): String = \"a\"\n  return maker(1)\n}\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "private function imported from a different package's file is invisible",
			files: map[string]string{
				"a_other.kt":  "package p.a\nprivate fun maker(x: String): String = x\n",
				"b_caller.kt": "package p.b\nimport p.c.maker\nfun use(x: String) = maker(x)\n",
				"c_decl.kt":   "package p.c\nfun maker(x: String): String? = null\n",
			},
			call: "maker(x)",
			want: "String?",
		},
		{
			name: "same-file private function stays visible",
			files: map[string]string{
				"a_caller.kt": "package p\nprivate fun maker(x: Int): String = \"a\"\nfun use() = maker(1)\n",
				"z_other.kt":  "package p\nprivate fun maker(x: Int): String? = null\n",
			},
			call: "maker(1)",
			want: "String",
		},
		{
			name: "same-file overloads with different nullability decline",
			files: map[string]string{
				"a_caller.kt": "package p\nprivate fun maker(x: String): String? = null\nprivate fun maker(x: Int): String = \"a\"\nfun use(x: String) = maker(x)\n",
			},
			call: "maker(x)",
			want: "",
		},
		{
			name: "visible overloads across files with different nullability decline",
			files: map[string]string{
				"a_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nfun maker(x: Int): String = \"a\"\n",
			},
			call: "maker(x)",
			want: "",
		},
		{
			name: "visible candidate without a declared return type declines",
			files: map[string]string{
				"a_caller.kt": "package p\nfun use(x: String) = maker(x)\n",
				"m_decl.kt":   "package p\nfun maker(x: String): String = x\n",
				"z_other.kt":  "package p\nfun maker(x: Int) = x.toString()\n",
			},
			call: "maker(x)",
			want: "",
		},
		{
			name: "agreeing overloads keep the shared return type",
			files: map[string]string{
				"a_caller.kt": "package p\nfun use(x: String) = maker(x)\n",
				"m_decl.kt":   "package p\nfun maker(x: String): String = x\n",
				"z_other.kt":  "package p\nfun maker(x: Int): String = \"a\"\n",
			},
			call: "maker(x)",
			want: "String",
		},
		{
			name: "unique public function in another file resolves",
			files: map[string]string{
				"a_caller.kt": "package p\nfun use() = maker(1)\n",
				"z_decl.kt":   "package p\nfun maker(x: Int): String = \"a\"\n",
			},
			call: "maker(1)",
			want: "String",
		},
		{
			name: "extension in another file with a non-null bare-name overload declines",
			files: map[string]string{
				"a_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nclass Box\nfun Box.maker(x: String): String = x\n",
			},
			call: "maker(x)",
			want: "",
		},
		{
			name: "private extension in another file is invisible",
			files: map[string]string{
				"a_caller.kt": "package p\nfun maker(x: String): String? = null\nfun use(x: String) = maker(x)\n",
				"z_other.kt":  "package p\nclass Box\nprivate fun Box.maker(x: String): String = x\n",
			},
			call: "maker(x)",
			want: "String?",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var files []*scanner.File
			var caller *scanner.File
			for name, src := range tc.files {
				f := parseTempFile(t, dir, name, src)
				files = append(files, f)
				if flatFirstOfTypeWithText(f, "call_expression", tc.call) != 0 && hasCallerUse(f) {
					caller = f
				}
			}
			if caller == nil {
				t.Fatalf("no caller file contains %q", tc.call)
			}
			resolver := NewResolver()
			resolver.IndexFilesParallel(files, 2)
			call := flatFirstOfTypeWithText(caller, "call_expression", tc.call)
			got := renderResolved(resolver.ResolveFlatNode(call, caller))
			if got != tc.want {
				t.Fatalf("ResolveFlatNode(%s) = %q, want %q", tc.call, got, tc.want)
			}
		})
	}
}

// TestNavigationExtensionReturnType_PrivateInOtherFile checks the receiver
// extension lookup ignores a private extension declared in another file.
func TestNavigationExtensionReturnType_PrivateInOtherFile(t *testing.T) {
	dir := t.TempDir()
	caller := parseTempFile(t, dir, "a_caller.kt", "package p\nclass Box\nfun Box.maker(): String? = null\nfun use(b: Box) = b.maker()\n")
	other := parseTempFile(t, dir, "z_other.kt", "package p\nprivate fun Box.maker(): String = \"a\"\n")
	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{caller, other}, 1)
	call := flatFirstOfTypeWithText(caller, "call_expression", "b.maker()")
	if got := renderResolved(resolver.ResolveFlatNode(call, caller)); got != "String?" {
		t.Fatalf("b.maker() = %q, want %q", got, "String?")
	}
}

func hasCallerUse(f *scanner.File) bool {
	return flatFirstOfTypeWithText(f, "simple_identifier", "use") != 0
}

func renderResolved(t *ResolvedType) string {
	if t == nil || t.Kind == TypeUnknown {
		return ""
	}
	if t.IsNullable() {
		return t.Name + "?"
	}
	return t.Name
}

// TestCallReturnType_ReindexReplacesFileDeclarations checks a re-indexed file
// replaces its earlier declarations instead of adding a conflicting candidate.
func TestCallReturnType_ReindexReplacesFileDeclarations(t *testing.T) {
	dir := t.TempDir()
	caller := parseTempFile(t, dir, "a_caller.kt", "package p\nfun use() = maker(1)\n")
	decl := parseTempFile(t, dir, "z_decl.kt", "package p\nfun maker(x: Int): String? = null\n")
	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{caller, decl}, 1)
	decl = parseTempFile(t, dir, "z_decl.kt", "package p\nfun maker(x: Int): String = \"a\"\n")
	resolver.IndexFilesParallel([]*scanner.File{caller, decl}, 1)
	call := flatFirstOfTypeWithText(caller, "call_expression", "maker(1)")
	if got := renderResolved(resolver.ResolveFlatNode(call, caller)); got != "String" {
		t.Fatalf("maker(1) after re-index = %q, want %q", got, "String")
	}
}
