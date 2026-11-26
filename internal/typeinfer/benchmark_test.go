package typeinfer

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseFixtureTI(b *testing.B, relPath string) *scanner.File {
	b.Helper()
	abs, err := filepath.Abs(relPath)
	if err != nil {
		b.Fatalf("cannot resolve path %s: %v", relPath, err)
	}
	if _, err := os.Stat(abs); err != nil {
		b.Skipf("fixture not found: %s", abs)
	}
	f, err := scanner.ParseFile(context.Background(), abs)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", abs, err)
	}
	return f
}

func BenchmarkIndexFileParallel_Small(b *testing.B) {
	file := parseFixtureTI(b, "../../tests/fixtures/positive/style/WildcardImport.kt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}

func BenchmarkIndexFileParallel_Large(b *testing.B) {
	file := parseFixtureTI(b, "../../tests/fixtures/positive/complexity/LargeClass.kt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}

func BenchmarkIndexFileParallel_HeavyDeclarations(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	for i := 0; i < 120; i++ {
		sb.WriteString("class Outer")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" {\n")
		sb.WriteString("    val p")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(": String = \"x\"\n")
		sb.WriteString("    companion object {\n")
		sb.WriteString("        fun create")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(): String = \"x\"\n")
		sb.WriteString("        val flag")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(": Boolean = true\n")
		sb.WriteString("    }\n")
		sb.WriteString("    fun method")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(): Int = 1\n")
		sb.WriteString("}\n\n")
	}
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "heavy.kt")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}

func BenchmarkIndexFileParallel_HeavyImportsAndDeclarations(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("package bench.deep\n\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("import bench.pkg")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(".Thing")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	for i := 0; i < 120; i++ {
		sb.WriteString("sealed interface Api")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" {\n")
		sb.WriteString("    public abstract fun run")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(input: String): Result<String>\n")
		sb.WriteString("}\n")
		sb.WriteString("data class Impl")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(private val value")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(": String) : Api")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" {\n")
		sb.WriteString("    override fun run")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(input: String): Result<String> = TODO()\n")
		sb.WriteString("}\n\n")
	}
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "heavy_imports.kt")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}

func BenchmarkIndexFileParallel_Sample(b *testing.B) {
	file := parseFixtureTI(b, "../../tests/fixtures/Sample.kt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}

func BenchmarkIndexFilesParallel(b *testing.B) {
	// Collect several fixture files and benchmark parallel indexing
	fixtureDir, err := filepath.Abs("../../tests/fixtures/positive/style")
	if err != nil {
		b.Fatal(err)
	}
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		b.Skipf("cannot read fixture dir: %v", err)
	}

	var files []*scanner.File
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".kt" {
			continue
		}
		f, err := scanner.ParseFile(context.Background(), filepath.Join(fixtureDir, e.Name()))
		if err != nil {
			continue
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		b.Skip("no fixtures found")
	}
	b.Logf("indexing %d files", len(files))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewResolver()
		r.IndexFilesParallel(files, 4)
	}
}

// BenchmarkIndexFileParallel_ConstructorParamInitializers covers data-holder
// classes whose property initializers and init blocks read primary-
// constructor parameters, which get an initializer scope each (#1673).
func BenchmarkIndexFileParallel_ConstructorParamInitializers(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	for i := 0; i < 60; i++ {
		n := strconv.Itoa(i)
		sb.WriteString("class Model" + n + "(")
		for p := 0; p < 8; p++ {
			sb.WriteString("p" + strconv.Itoa(p) + ": String?, ")
		}
		sb.WriteString("val id: Long) {\n")
		for p := 0; p < 8; p++ {
			ps := strconv.Itoa(p)
			sb.WriteString("    val v" + ps + ": String = p" + ps + " ?: \"\"\n")
		}
		sb.WriteString("    val lazyValue by lazy { p0 ?: p1 }\n")
		sb.WriteString("    init {\n        require(id > 0)\n        val local = p2\n    }\n")
		sb.WriteString("    fun describe(): String = v0 + v1\n}\n\n")
	}
	tmp := b.TempDir() + "/Models.kt"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	file := parseFixtureTI(b, tmp)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexFileParallel(file)
	}
}
