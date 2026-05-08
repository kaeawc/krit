package android

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResourceGraph_ValuesReferenceAffectsLayout(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	stringsPath := filepath.Join(resDir, "values", "strings.xml")
	layoutPath := filepath.Join(resDir, "layout", "main.xml")
	writeGraphFile(t, stringsPath, `<resources><string name="title">Title</string></resources>`)
	writeGraphFile(t, layoutPath, `<TextView android:text="@string/title"/>`)

	graph, err := (XMLResourceGraphBuilder{}).Build(root, []string{resDir}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := (GraphResolver{Graph: graph}).AffectedFiles([]string{stringsPath})
	want := []string{layoutPath, stringsPath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("affected = %v, want %v", got, want)
	}
}

func TestResourceGraph_UnrelatedValueDoesNotAffectLayout(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	stringsPath := filepath.Join(resDir, "values", "strings.xml")
	colorsPath := filepath.Join(resDir, "values", "colors.xml")
	layoutPath := filepath.Join(resDir, "layout", "main.xml")
	writeGraphFile(t, stringsPath, `<resources><string name="title">Title</string></resources>`)
	writeGraphFile(t, colorsPath, `<resources><color name="brand">#fff</color></resources>`)
	writeGraphFile(t, layoutPath, `<TextView android:text="@string/title"/>`)

	graph, err := (XMLResourceGraphBuilder{}).Build(root, []string{resDir}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := (GraphResolver{Graph: graph}).AffectedFiles([]string{colorsPath})
	want := []string{colorsPath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("affected = %v, want %v", got, want)
	}
}

func TestResourceGraph_StyleParentAndCycle(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	stylesPath := filepath.Join(resDir, "values", "styles.xml")
	layoutPath := filepath.Join(resDir, "layout", "main.xml")
	writeGraphFile(t, stylesPath, `<resources>
<style name="Parent" parent="Child"/>
<style name="Child" parent="Parent"/>
</resources>`)
	writeGraphFile(t, layoutPath, `<TextView style="@style/Parent"/>`)

	graph, err := (XMLResourceGraphBuilder{}).Build(root, []string{resDir}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := (GraphResolver{Graph: graph}).AffectedFiles([]string{stylesPath})
	want := []string{layoutPath, stylesPath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("affected = %v, want %v", got, want)
	}
}

func TestCachedResourceGraphBuilder_UsesCache(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	layoutPath := filepath.Join(resDir, "layout", "main.xml")
	writeGraphFile(t, layoutPath, `<LinearLayout/>`)

	builder := &countingGraphBuilder{graph: &ResourceGraph{Files: map[string]ResourceFileNode{layoutPath: {Path: layoutPath}}}}
	cache := &memoryResourceGraphCache{}
	first, err := (CachedResourceGraphBuilder{Builder: builder, Cache: cache}).Build(root, []string{resDir}, nil)
	if err != nil {
		t.Fatalf("first Build: %v", err)
	}
	second, err := (CachedResourceGraphBuilder{Builder: builder, Cache: cache}).Build(root, []string{resDir}, nil)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if builder.calls != 1 {
		t.Fatalf("builder calls = %d, want 1", builder.calls)
	}
	if first != second {
		t.Fatal("expected second build to return cached graph")
	}
}

func TestDiskResourceGraphCache_RoundTripStructKeys(t *testing.T) {
	root := t.TempDir()
	graph := &ResourceGraph{
		Files: map[string]ResourceFileNode{
			"/res/values/strings.xml": {
				Path:    "/res/values/strings.xml",
				Defines: []ResourceID{{Type: "string", Name: "title"}},
			},
		},
		DefinedIn: map[ResourceID][]string{
			{Type: "string", Name: "title"}: {"/res/values/strings.xml"},
		},
		ReverseDeps: map[string][]string{
			"/res/values/strings.xml": {"/res/layout/main.xml"},
		},
	}
	cache := DiskResourceGraphCache{}
	if err := cache.Save(root, "key", graph); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok := cache.Load(root, "key")
	if !ok {
		t.Fatal("Load returned ok=false")
	}
	if !reflect.DeepEqual(got, graph) {
		t.Fatalf("graph = %#v, want %#v", got, graph)
	}
}

func TestDiskResourceGraphCache_KeyMismatchMisses(t *testing.T) {
	root := t.TempDir()
	cache := DiskResourceGraphCache{}
	if err := cache.Save(root, "key-a", &ResourceGraph{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := cache.Load(root, "key-b"); ok {
		t.Fatal("Load returned hit for different key")
	}
}

type countingGraphBuilder struct {
	graph *ResourceGraph
	calls int
}

func (b *countingGraphBuilder) Build(root string, resDirs []string, hashes map[string]string) (*ResourceGraph, error) {
	b.calls++
	return b.graph, nil
}

type memoryResourceGraphCache struct {
	data map[string]*ResourceGraph
}

func (c *memoryResourceGraphCache) Load(root, key string) (*ResourceGraph, bool) {
	if c.data == nil {
		return nil, false
	}
	graph, ok := c.data[key]
	return graph, ok
}

func (c *memoryResourceGraphCache) Save(root, key string, graph *ResourceGraph) error {
	if c.data == nil {
		c.data = make(map[string]*ResourceGraph)
	}
	c.data[key] = graph
	return nil
}

func writeGraphFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
