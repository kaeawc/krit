package scanner

import (
	"context"
	"testing"
)

func jumpExpressionTexts(t *testing.T, src string) []string {
	t.Helper()
	parser := GetKotlinParser()
	defer PutKotlinParser(parser)
	ft, err := ParseKotlinFlat(context.Background(), parser, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, j := range ft.NodesOfType(internNodeType("jump_expression")) {
		out = append(out, src[ft.StartBytes[j]:ft.EndBytes[j]])
	}
	return out
}

func TestReturnOperandStartingWithE(t *testing.T) {
	got := jumpExpressionTexts(t, "fun a(e: Int): Int { return e }\nfun b() { return emptyList() }\nfun c(x: Int) { return x }\n")
	want := []string{"return e", "return emptyList()", "return x"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestBareReturnKeepsFollowingStatement(t *testing.T) {
	got := jumpExpressionTexts(t, "fun a() {\n  return\n  emit()\n}\nfun b(x: Boolean) { if (x) return else emit() }\n")
	want := []string{"return", "return"}
	if len(got) != len(want) || got[0] != "return" || got[1] != "return" {
		t.Fatalf("got %q, want %q", got, want)
	}
}
