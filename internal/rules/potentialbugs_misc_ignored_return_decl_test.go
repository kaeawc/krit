package rules

import (
	"math/rand"
	"regexp"
	"testing"
)

// TestTextDeclaresFunctionTypedMatchesRegexp checks the hand-written matcher against the regexp it replaced.
func TestTextDeclaresFunctionTypedMatchesRegexp(t *testing.T) {
	const name = "sink"
	re := regexp.MustCompile(`(?:^|[\s(,])` + regexp.QuoteMeta(name) + `\s*:\s*[^=]*->`)
	fixed := []string{
		"sink: (Int) -> Unit",
		"val sink: (Int) -> Unit = {}",
		"fun f(a: Int, sink : (String) -> Unit)",
		"fun f(sink: Int = 1, other: (A) -> B)",
		"xsink: (Int) -> Unit",
		"sink:\n  (Int) -> Unit",
		"sink = 1 -> 2",
		"sink",
		"",
		"sink: Int, sink2: (A) -> B",
		"sink: Int\nsink: (A) -> B",
		"(sink: () -> Unit)",
	}
	for _, text := range fixed {
		if got, want := textDeclaresFunctionTyped([]byte(text), name, true), re.MatchString(text); got != want {
			t.Errorf("%q: got %v, regexp %v", text, got, want)
		}
	}
	alphabet := []string{"sink", " ", ":", "-", ">", "=", "(", ",", "\n", "x", "->", "Int", "\t"}
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 20000; n++ {
		var text string
		for i := rng.Intn(10); i > 0; i-- {
			text += alphabet[rng.Intn(len(alphabet))]
		}
		got := textDeclaresFunctionTyped([]byte(text), name, true)
		if want := re.MatchString(text); got != want {
			t.Fatalf("%q: got %v, regexp %v", text, got, want)
		}
		if re.MatchString(text) && !textDeclaresFunctionTyped([]byte(text), name, false) {
			t.Fatalf("%q: relaxed check rejected a boundary match", text)
		}
	}
}
