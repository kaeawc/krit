package rules

import (
	"context"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestLintCallAtMemoMatchesCompute checks the pass-memoized lintCallAt against computeLintCallAt on every call of
// the Kotlin and Java fixtures, on the first lookup (which fills the memo) and the second (which reads it).
func TestLintCallAtMemoMatchesCompute(t *testing.T) {
	calls := 0
	err := filepath.WalkDir(filepath.Join("..", "..", "tests", "fixtures"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var file *scanner.File
		switch {
		case strings.HasSuffix(path, ".kt"):
			file, err = scanner.ParseFile(context.Background(), path)
		case strings.HasSuffix(path, ".java"):
			file, err = scanner.ParseJavaFile(context.Background(), path)
		default:
			return nil
		}
		if err != nil {
			return err
		}
		file.SetPassMemo(scanner.NewPassMemo())
		defer file.SetPassMemo(nil)
		for idx := uint32(1); int(idx) < file.FlatTree.Len(); idx++ {
			switch file.FlatType(idx) {
			case "call_expression", "method_invocation", "object_creation_expression":
			default:
				continue
			}
			calls++
			want, wantOK := computeLintCallAt(file, idx)
			for pass := range 2 {
				got, ok := lintCallAt(file, idx)
				if ok != wantOK || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s node %d, lookup %d: lintCallAt = %+v, %v; want %+v, %v", path, idx, pass, got, ok, want, wantOK)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("no calls found in the fixtures")
	}
}
