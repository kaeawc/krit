package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseCoroutinesInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// firstFunctionDeclaration returns the first function_declaration node
// in the file (depth-first, document order).
func firstFunctionDeclaration(file *scanner.File) uint32 {
	var found uint32
	file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
		if found == 0 {
			found = idx
		}
	})
	return found
}

// Locks the structural extension-function signal used by
// InjectDispatcher: tree-sitter emits a `.` token direct child of the
// function_declaration only for extension receivers, across plain,
// nullable, and generic receiver shapes. Plain and member functions
// must report false.
func TestFlatFunctionDeclarationIsExtension(t *testing.T) {
	cases := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "plain top-level",
			code: "package p\nfun loadTopLevel() { x() }\n",
			want: false,
		},
		{
			name: "extension with simple receiver",
			code: "package p\nfun String.loadExt() { x() }\n",
			want: true,
		},
		{
			name: "extension with nullable receiver",
			code: "package p\nfun String?.loadExt() { x() }\n",
			want: true,
		},
		{
			name: "extension with generic receiver",
			code: "package p\nfun <T> T.loadExt() { x() }\n",
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := parseCoroutinesInline(t, tc.code)
			fn := firstFunctionDeclaration(file)
			if fn == 0 {
				t.Fatal("no function_declaration found")
			}
			if got := flatFunctionDeclarationIsExtension(file, fn); got != tc.want {
				t.Fatalf("flatFunctionDeclarationIsExtension = %v, want %v", got, tc.want)
			}
		})
	}
}

// Confirms the host check that gates InjectDispatcher: a dispatcher
// usage has an injectable host only when enclosed by a
// class/object/companion, regardless of whether the enclosing function
// is an extension.
func TestInjectDispatcherHasInjectableHost(t *testing.T) {
	cases := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "top-level function",
			code: "package p\nfun f() { withContext(Dispatchers.IO) {} }\n",
			want: false,
		},
		{
			name: "top-level extension function",
			code: "package p\nfun String.f() { withContext(Dispatchers.IO) {} }\n",
			want: false,
		},
		{
			name: "class member",
			code: "package p\nclass C { fun f() { withContext(Dispatchers.IO) {} } }\n",
			want: true,
		},
		{
			name: "object member",
			code: "package p\nobject O { fun f() { withContext(Dispatchers.IO) {} } }\n",
			want: true,
		},
		{
			name: "member extension function",
			code: "package p\nclass C { fun String.f() { withContext(Dispatchers.IO) {} } }\n",
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := parseCoroutinesInline(t, tc.code)
			var usage uint32
			file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
				if usage == 0 && file.FlatNodeText(idx) == "Dispatchers.IO" {
					usage = idx
				}
			})
			if usage == 0 {
				t.Fatal("Dispatchers.IO navigation_expression not found")
			}
			if got := injectDispatcherHasInjectableHost(file, usage); got != tc.want {
				t.Fatalf("injectDispatcherHasInjectableHost = %v, want %v", got, tc.want)
			}
		})
	}
}
