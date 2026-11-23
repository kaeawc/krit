package pipeline

import (
	"context"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestCrossFileStructuralFingerprintParallelMatchesSerial(t *testing.T) {
	var files []*scanner.File
	var serial []string
	for i, src := range []string{
		"class A { fun a() = 1 }",
		"class B(val a: A) { fun b() = a.a() }",
		"fun top(b: B): Int = b.b()",
		"object C { const val X = 1 }",
	} {
		parser := scanner.GetKotlinParser()
		ft, err := scanner.ParseKotlinFlat(context.Background(), parser, []byte(src))
		scanner.PutKotlinParser(parser)
		if err != nil {
			t.Fatal(err)
		}
		f := &scanner.File{Path: string(rune('a'+i)) + ".kt", Language: scanner.LangKotlin, Content: []byte(src), FlatTree: ft}
		files = append(files, f)
		fresh := &scanner.File{Path: f.Path, Language: f.Language, Content: f.Content, FlatTree: ft}
		serial = append(serial, scanner.FileStructuralFingerprint(fresh))
	}
	_ = crossFileStructuralFingerprint(files, nil, nil, nil)
	for i, f := range files {
		if got := scanner.FileStructuralFingerprint(f); got != serial[i] {
			t.Fatalf("file %d: prewarmed fp %s, serial %s", i, got, serial[i])
		}
	}
}
