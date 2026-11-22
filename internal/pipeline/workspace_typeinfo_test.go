package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestWorkspaceState_FileTypeInfoCache_SameFileLookalikes(t *testing.T) {
	dir := t.TempDir()
	workspace := NewWorkspaceState(dir)
	cacheDir := t.TempDir()
	declarationPath := filepath.Join(dir, "A.kt")
	usePath := filepath.Join(dir, "B.kt")
	parse := func(path, source string) *scanner.File {
		t.Helper()
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	file := parse(usePath, `package example
class Context
fun read(items: List<String>) = items.firstOrNull()
fun local(context: Context) = context.getSystemService("service")
`)
	cached := typeinfer.IndexFileParallel(file)
	workspace.StoreFileTypeInfo(usePath, cached)
	snapshot := func() []byte {
		t.Helper()
		types := []map[string]*typeinfer.ResolvedType{cached.Functions, cached.TypeAliases}
		var visitScope func(*typeinfer.ScopeTable)
		visitScope = func(scope *typeinfer.ScopeTable) {
			types = append(types, scope.Entries, scope.SmartCastTypes)
			for _, child := range scope.Children {
				visitScope(child)
			}
		}
		visitScope(cached.RootScope)
		data, err := json.Marshal(types)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	before := snapshot()
	for scan, name := range []string{"List", "Lst", "List"} {
		declaration := parse(declarationPath, "package example\nclass "+name+"\n")
		workspace.Invalidate(declarationPath)
		resolver := typeinfer.NewResolver()
		resolver.SetResidentCache(workspace)
		hits, misses := resolver.IndexFilesParallelCachedWithTracker([]*scanner.File{declaration, file}, 1, cacheDir, nil)
		wantHits, wantMisses := 1, 1
		if scan == 2 {
			wantHits, wantMisses = 2, 0
		}
		if hits != wantHits || misses != wantMisses {
			t.Fatalf("scan %d: hits %d, misses %d", scan, hits, misses)
		}
		for _, test := range []struct {
			call    string
			unknown bool
		}{
			{`items.firstOrNull()`, false},
			{`context.getSystemService("service")`, true},
		} {
			var callIdx uint32
			for index := uint32(1); index < uint32(file.FlatTree.Len()); index++ {
				if file.FlatType(index) == "call_expression" && file.FlatNodeText(index) == test.call {
					callIdx = index
					break
				}
			}
			if callIdx == 0 {
				t.Fatalf("missing call %s", test.call)
			}
			typ := resolver.ResolveFlatNode(callIdx, file)
			if (typ.Kind == typeinfer.TypeUnknown) != test.unknown || !test.unknown && !typ.IsNullable() {
				t.Fatalf("scan %d, %s: unexpected type %+v", scan, test.call, typ)
			}
		}
		if got, ok := workspace.LookupFileTypeInfo(usePath); !ok || got != cached {
			t.Fatal("unchanged resident entry lost pointer identity")
		}
		if !bytes.Equal(before, snapshot()) {
			t.Fatalf("scan %d mutated resident types", scan)
		}
	}
}

// TestWorkspaceState_FileTypeInfoCache_StoreAndLookup pins the
// resident cache contract: store + lookup returns the same pointer,
// distinct paths don't collide, idempotent stores keep the first
// pointer so identity-based comparisons survive across analyses.
func TestWorkspaceState_FileTypeInfoCache_StoreAndLookup(t *testing.T) {
	w := NewWorkspaceState("")

	if got, ok := w.LookupFileTypeInfo("/tmp/A.kt"); ok || got != nil {
		t.Fatalf("empty cache: got=%v ok=%v, want nil/false", got, ok)
	}

	first := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", first)
	got, ok := w.LookupFileTypeInfo("/tmp/A.kt")
	if !ok {
		t.Fatalf("post-store lookup: ok=false")
	}
	if got != first {
		t.Errorf("post-store lookup returned different pointer")
	}

	// Distinct path: must not collide.
	if _, ok := w.LookupFileTypeInfo("/tmp/B.kt"); ok {
		t.Errorf("LookupFileTypeInfo leaked across paths")
	}

	// Idempotent store: a later Store under the same path keeps the
	// first pointer (matches the ResidentFiles cache's identity
	// semantics — rules compare *FileTypeInfo by pointer).
	second := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", second)
	got2, _ := w.LookupFileTypeInfo("/tmp/A.kt")
	if got2 != first {
		t.Errorf("second Store replaced pointer; want stable identity")
	}
}

// TestWorkspaceState_FileTypeInfoCache_InvalidateDropsEntry verifies
// the watcher hook: Invalidate(path) clears the typeInfo entry
// alongside the parsed-tree entry, so the next analyze re-indexes
// the file. Without this hook a stale FileTypeInfo would survive
// across edits the watcher saw.
func TestWorkspaceState_FileTypeInfoCache_InvalidateDropsEntry(t *testing.T) {
	w := NewWorkspaceState("")
	info := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", info)
	w.Invalidate("/tmp/A.kt")
	if got, ok := w.LookupFileTypeInfo("/tmp/A.kt"); ok || got != nil {
		t.Errorf("post-Invalidate lookup: got=%v ok=%v, want nil/false", got, ok)
	}
}

// TestWorkspaceState_FileTypeInfoCache_NilSafety mirrors the safety
// contract the rest of WorkspaceState's resident caches honor.
func TestWorkspaceState_FileTypeInfoCache_NilSafety(t *testing.T) {
	var w *WorkspaceState
	if got, ok := w.LookupFileTypeInfo("k"); ok || got != nil {
		t.Errorf("nil receiver: got=%v ok=%v, want nil/false", got, ok)
	}
	w.StoreFileTypeInfo("k", &typeinfer.FileTypeInfo{}) // must not panic

	w = NewWorkspaceState("")
	w.StoreFileTypeInfo("", &typeinfer.FileTypeInfo{}) // path-empty store is a no-op via normalizeKey
	w.StoreFileTypeInfo("/p.kt", nil)                  // nil info is a no-op
	if got, ok := w.LookupFileTypeInfo("/p.kt"); ok || got != nil {
		t.Errorf("Store(nil) leaked into cache: got=%v ok=%v", got, ok)
	}
}
