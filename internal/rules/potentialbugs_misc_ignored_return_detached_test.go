package rules

import (
	"math/rand/v2"
	"regexp"
	"testing"
)

// detachedReturnRegex is the pattern endsWithDetachedReturn replaced.
var detachedReturnRegex = regexp.MustCompile(`(?:^|[^\w@.])return(?:@\w+)?[ \t]+$`)

func TestEndsWithDetachedReturnMatchesRegex(t *testing.T) {
	cases := []string{
		"", " ", "return", "return ", "return\t", "  return ", "x = if (a) return ", "return@forEach ",
		"return@ ", "return@a_1 ", "@return ", ".return ", "xreturn ", "_return ", "1return ", "(return ",
		"return@forEach.x ", "return@forEachreturn ", "return@xreturn ", "areturn@l ", "é return ", "éreturn ",
		"return  \t ", "return\n", "return@l", "returns ", "retur n ", "return return ", "\xffreturn ",
	}
	for _, c := range cases {
		if got, want := endsWithDetachedReturn([]byte(c)), detachedReturnRegex.MatchString(c); got != want {
			t.Errorf("endsWithDetachedReturn(%q) = %v, regex = %v", c, got, want)
		}
	}
	alphabet := []string{"return", "return", "@", ".", " ", "\t", "a", "_", "1", "é", "(", "\xff", "re", "turn"}
	rng := rand.New(rand.NewPCG(1664, 1))
	for range 200_000 {
		var b []byte
		for n := rng.IntN(6); n >= 0; n-- {
			b = append(b, alphabet[rng.IntN(len(alphabet))]...)
		}
		if got, want := endsWithDetachedReturn(b), detachedReturnRegex.Match(b); got != want {
			t.Fatalf("endsWithDetachedReturn(%q) = %v, regex = %v", b, got, want)
		}
	}
}
