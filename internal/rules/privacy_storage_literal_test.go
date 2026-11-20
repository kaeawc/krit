package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestPlainFileWriteOfSensitiveLiteralContent(t *testing.T) {
	cases := []struct {
		literal string
		want    string
	}{
		{`"$tokenHash.cache"`, ".cache"},
		{`"user${id}token.txt"`, "usertoken.txt"},
		{`"C:\\auth\\data.txt"`, `C:\\auth\\data.txt`},
		{`"\u0074oken.txt"`, `\u0074oken.txt`},
	}
	for _, testCase := range cases {
		t.Run(testCase.literal, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "literal.kt")
			if err := os.WriteFile(path, []byte("val path = "+testCase.literal+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			var literal uint32
			file.FlatWalkNodes(0, "string_literal", func(node uint32) {
				if literal == 0 {
					literal = node
				}
			})
			if literal == 0 {
				t.Fatal("missing string literal")
			}
			if got := stringLiteralContent(file, literal); got != testCase.want {
				t.Fatalf("stringLiteralContent = %q, want %q", got, testCase.want)
			}
		})
	}
}
