package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helperParseFixture parses a fixture file relative to the repo root.
func helperParseFixture(b *testing.B, relPath string) *File {
	b.Helper()
	// Resolve from the package directory to repo root (internal/scanner -> ../..)
	abs, err := filepath.Abs(relPath)
	if err != nil {
		b.Fatalf("cannot resolve path %s: %v", relPath, err)
	}
	f, err := ParseFile(abs)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", abs, err)
	}
	return f
}

func BenchmarkParseFile(b *testing.B) {
	path, err := filepath.Abs("../../tests/fixtures/positive/style/WildcardImport.kt")
	if err != nil {
		b.Fatal(err)
	}
	// Verify the file exists before benchmarking
	if _, err := os.Stat(path); err != nil {
		b.Skipf("fixture not found: %s", path)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFile_Large(b *testing.B) {
	path, err := filepath.Abs("../../tests/fixtures/positive/complexity/LargeClass.kt")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		b.Skipf("fixture not found: %s", path)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFile_Inline(b *testing.B) {
	// Parse a synthetic moderately-sized Kotlin source from memory
	// to measure parsing without file I/O.
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	for i := 0; i < 50; i++ {
		sb.WriteString("import kotlin.collections.*\n")
	}
	sb.WriteString("\nclass BenchClass {\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("    fun method" + string(rune('A'+i%26)) + "() {\n")
		sb.WriteString("        val x = listOf(1, 2, 3).map { it * 2 }\n")
		sb.WriteString("        println(x)\n")
		sb.WriteString("    }\n\n")
	}
	sb.WriteString("}\n")

	src := sb.String()
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "bench.kt")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseFile(tmpFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFile_WithPool(b *testing.B) {
	path, err := filepath.Abs("../../tests/fixtures/positive/style/WildcardImport.kt")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		b.Skipf("fixture not found: %s", path)
	}
	// Warm the pool with one parser
	p := GetKotlinParser()
	PutKotlinParser(p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLineOffsets(b *testing.B) {
	f := helperParseFixture(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache to force recomputation
		f.lineOffsets = nil
		_ = f.LineOffsets()
	}
}

func BenchmarkBuildSuppressionIndex(b *testing.B) {
	f := helperParseFixture(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildSuppressionIndexFlat(f.FlatTree, f.Content)
	}
}

// benchParseIdentifiers builds a synthetic Kotlin file with repeated
// identifiers, parses it, and returns the file plus all simple_identifier
// node indices. Shared across node-text benchmarks.
func benchParseIdentifiers(b *testing.B) (*File, []uint32) {
	b.Helper()
	var src strings.Builder
	src.WriteString("package bench\n\n")
	src.WriteString("class Demo {\n")
	for i := 0; i < 200; i++ {
		src.WriteString("    fun repeated(sample: String): String {\n")
		src.WriteString("        val local = sample.trim()\n")
		src.WriteString("        return sample + local + sample + local\n")
		src.WriteString("    }\n")
	}
	src.WriteString("}\n")

	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "bench.kt")
	if err := os.WriteFile(tmpFile, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}

	f, err := ParseFile(tmpFile)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", tmpFile, err)
	}

	var identifiers []uint32
	FlatWalkNodes(f.FlatTree, "simple_identifier", func(idx uint32) {
		identifiers = append(identifiers, idx)
	})
	if len(identifiers) == 0 {
		b.Fatal("expected benchmark file to contain identifiers")
	}
	return f, identifiers
}

func BenchmarkNodeTextRepeatedIdentifiers(b *testing.B) {
	f, identifiers := benchParseIdentifiers(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var total int
		for _, idx := range identifiers {
			total += len(f.FlatNodeText(idx))
		}
		if total == 0 {
			b.Fatal("expected repeated identifier text")
		}
	}
}

func BenchmarkFlatNodeTextEquals(b *testing.B) {
	f, identifiers := benchParseIdentifiers(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var hits int
		for _, idx := range identifiers {
			if f.FlatNodeTextEquals(idx, "sample") {
				hits++
			}
		}
		if hits == 0 {
			b.Fatal("expected hits for 'sample'")
		}
	}
}

func BenchmarkFlatHasModifier(b *testing.B) {
	var src strings.Builder
	src.WriteString("package bench\n\n")
	for i := 0; i < 100; i++ {
		src.WriteString("    abstract override fun method() {}\n")
		src.WriteString("    open fun other() {}\n")
		src.WriteString("    private val x = 1\n")
	}

	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "bench.kt")
	if err := os.WriteFile(tmpFile, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}

	f, err := ParseFile(tmpFile)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", tmpFile, err)
	}

	var decls []uint32
	FlatWalkNodes(f.FlatTree, "function_declaration", func(idx uint32) {
		decls = append(decls, idx)
	})
	FlatWalkNodes(f.FlatTree, "property_declaration", func(idx uint32) {
		decls = append(decls, idx)
	})
	if len(decls) == 0 {
		b.Fatal("expected declarations")
	}

	modifiers := []string{"abstract", "override", "open", "private", "public", "internal"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var hits int
		for _, idx := range decls {
			for _, mod := range modifiers {
				if f.FlatHasModifier(idx, mod) {
					hits++
				}
			}
		}
		if hits == 0 {
			b.Fatal("expected modifier hits")
		}
	}
}

func BenchmarkFlatNodeString(b *testing.B) {
	f, identifiers := benchParseIdentifiers(b)
	pool := NewStringPool()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var total int
		for _, idx := range identifiers {
			total += len(FlatNodeString(f.FlatTree, idx, f.Content, pool))
		}
		if total == 0 {
			b.Fatal("expected interned identifier text")
		}
	}
}

// BenchmarkNodeTextVsZeroCopy compares allocation counts between the old
// FlatNodeText path (allocates per call) and the zero-copy alternatives.
func BenchmarkNodeTextVsZeroCopy(b *testing.B) {
	var src strings.Builder
	src.WriteString("package bench\n\n")
	src.WriteString("import kotlin.collections.*\n\n")
	for i := 0; i < 50; i++ {
		src.WriteString("abstract class Service {\n")
		src.WriteString("    override fun process(modifier: String): String {\n")
		src.WriteString("        val result = modifier.trim()\n")
		src.WriteString("        return result\n")
		src.WriteString("    }\n")
		src.WriteString("    open fun validate(input: String) {}\n")
		src.WriteString("    private fun helper() {}\n")
		src.WriteString("}\n\n")
	}

	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "bench.kt")
	if err := os.WriteFile(tmpFile, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}

	f, err := ParseFile(tmpFile)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", tmpFile, err)
	}

	var allNodes []uint32
	for i := range f.FlatTree.Nodes {
		allNodes = append(allNodes, uint32(i))
	}

	modifiers := []string{"abstract", "override", "open", "private"}
	keywords := []string{"val", "var", "fun", "class", "interface", "return"}

	b.Run("FlatNodeText_allocating", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var hits int
			for _, idx := range allNodes {
				text := f.FlatNodeText(idx)
				for _, kw := range keywords {
					if text == kw {
						hits++
					}
				}
			}
			_ = hits
		}
	})

	b.Run("FlatNodeTextEquals_zero_copy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var hits int
			for _, idx := range allNodes {
				for _, kw := range keywords {
					if f.FlatNodeTextEquals(idx, kw) {
						hits++
					}
				}
			}
			_ = hits
		}
	})

	b.Run("FlatHasModifier_zero_copy", func(b *testing.B) {
		b.ReportAllocs()
		var decls []uint32
		FlatWalkNodes(f.FlatTree, "function_declaration", func(idx uint32) {
			decls = append(decls, idx)
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var hits int
			for _, idx := range decls {
				for _, mod := range modifiers {
					if f.FlatHasModifier(idx, mod) {
						hits++
					}
				}
			}
			_ = hits
		}
	})

	b.Run("FlatNodeString_interned", func(b *testing.B) {
		pool := NewStringPool()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var total int
			for _, idx := range allNodes {
				total += len(FlatNodeString(f.FlatTree, idx, f.Content, pool))
			}
			_ = total
		}
	})
}

func BenchmarkCollectKotlinFiles(b *testing.B) {
	dir, err := filepath.Abs("../../tests/fixtures")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		b.Skipf("fixtures dir not found: %s", dir)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CollectKotlinFiles([]string{dir}, nil)
	}
}
