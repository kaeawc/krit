package rename

import (
	"testing"
	"testing/quick"

	"github.com/kaeawc/krit/internal/scanner"
)

// rewriteImportLineFixture wraps a canned input through the rewriter
// without going through the full Apply pipeline. The input lives in
// file.Content; rng spans the import line; toFQN is the rename target.
func rewriteImportLineFixture(t *testing.T, content, toFQN string, lang scanner.Language) string {
	t.Helper()
	file := &scanner.File{Content: []byte(content), Language: lang}
	return rewriteImportLine(file, [2]int{0, len(content)}, toFQN)
}

func TestRewriteImportLine_KotlinNoSemi(t *testing.T) {
	got := rewriteImportLineFixture(t, "import com.foo.Old", "com.bar.New", scanner.LangKotlin)
	if got != "import com.bar.New" {
		t.Errorf("got %q", got)
	}
}

func TestRewriteImportLine_KotlinWithAlias(t *testing.T) {
	got := rewriteImportLineFixture(t, "import com.foo.Old as F", "com.bar.New", scanner.LangKotlin)
	if got != "import com.bar.New as F" {
		t.Errorf("alias not preserved: %q", got)
	}
}

func TestRewriteImportLine_JavaWithSemi(t *testing.T) {
	got := rewriteImportLineFixture(t, "import com.foo.Old;", "com.bar.New", scanner.LangJava)
	if got != "import com.bar.New;" {
		t.Errorf("got %q", got)
	}
}

func TestRewriteImportLine_JavaStatic(t *testing.T) {
	got := rewriteImportLineFixture(t, "import static com.foo.Old.X;", "com.bar.New.X", scanner.LangJava)
	if got != "import static com.bar.New.X;" {
		t.Errorf("static modifier not preserved: %q", got)
	}
}

func TestRewriteImportLine_TrimsSurroundingWhitespace(t *testing.T) {
	got := rewriteImportLineFixture(t, "  import com.foo.Old  ", "com.bar.New", scanner.LangKotlin)
	if got != "import com.bar.New" {
		t.Errorf("got %q", got)
	}
}

func TestRewriteImportLine_KotlinSemiPreserved(t *testing.T) {
	// Kotlin allows trailing ';'; the rewriter mirrors what the
	// original line had.
	got := rewriteImportLineFixture(t, "import com.foo.Old;", "com.bar.New", scanner.LangKotlin)
	if got != "import com.bar.New;" {
		t.Errorf("trailing ; not preserved: %q", got)
	}
}

func TestRewriteImportLine_RejectsInvalidRange(t *testing.T) {
	file := &scanner.File{Content: []byte("import com.foo.Old"), Language: scanner.LangKotlin}
	if got := rewriteImportLine(file, [2]int{-1, 5}, "X"); got != "" {
		t.Errorf("negative start should yield empty; got %q", got)
	}
	if got := rewriteImportLine(file, [2]int{0, 1000}, "X"); got != "" {
		t.Errorf("end past content should yield empty; got %q", got)
	}
	if got := rewriteImportLine(file, [2]int{5, 5}, "X"); got != "" {
		t.Errorf("zero-length should yield empty; got %q", got)
	}
}

// TestRewriteImportLine_NoTrailingWhitespaceInOutput is a quickcheck
// invariant: the rewriter never emits leading/trailing whitespace in
// its return value, regardless of how messy the input is.
func TestRewriteImportLine_NoTrailingWhitespaceInOutput(t *testing.T) {
	cfg := &quick.Config{MaxCount: 200}
	property := func(toFQNSeed uint8) bool {
		toFQN := "com.bar.NewName"
		_ = toFQNSeed
		// Mix of leading/trailing whitespace and trailing ';'.
		variants := []string{
			"  import com.foo.Old   ",
			"\timport com.foo.Old\t",
			"import com.foo.Old;",
			" import com.foo.Old ; ",
		}
		for _, v := range variants {
			out := rewriteImportLineFixture(t, v, toFQN, scanner.LangKotlin)
			if out == "" {
				return false
			}
			if out[0] == ' ' || out[0] == '\t' {
				return false
			}
			last := out[len(out)-1]
			if last == ' ' || last == '\t' {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, cfg); err != nil {
		t.Fatalf("quickcheck: %v", err)
	}
}
