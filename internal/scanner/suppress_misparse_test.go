package scanner

import (
	"strings"
	"testing"
)

// misparsedClass builds a class whose constructor layout makes the grammar split the declaration.
func misparsedClass(classAnnotations, header string) string {
	return classAnnotations + header + ` {
  fun before() {
    val a = 1
  }

  @Suppress("LongMethod")
  override fun attach() {
    val b = 2
  }

  fun plain() {
    val c = 3
  }
}

fun after() {
  val d = 4
}
`
}

func lineOf(t *testing.T, src, needle string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("%q not found", needle)
	return 0
}

var misparsedHeaders = map[string]string{
	"inject-newline":      "class Presenter\n@Inject\nconstructor(\n  private val a: Int,\n) : Contract.Presenter",
	"inject-same-line":    "class Presenter\n@Inject constructor(\n  private val a: Int,\n) : Contract.Presenter",
	"private-constructor": "class Presenter\nprivate constructor(\n  private val a: Int,\n) : Contract.Presenter",
	"inline-inject":       "class Presenter @Inject constructor(private val a: Int) : Contract.Presenter",
	"delegation":          "class Presenter\n@Inject\nconstructor(\n  private val a: Int,\n  private val l: Lazy<Foo>,\n) : Contract.Presenter, Other by a",
}

func TestSuppressMisparsedClass_MemberLevel(t *testing.T) {
	for name, header := range misparsedHeaders {
		t.Run(name, func(t *testing.T) {
			src := misparsedClass("@ContributesBinding(Scope::class)\n", header)
			sf := BuildSuppressionFilter(parsedFileForFilter(t, "P.kt", src), nil, nil, "")
			if !sf.IsSuppressed("LongMethod", "", lineOf(t, src, "override fun attach")) {
				t.Error("member @Suppress must suppress its function")
			}
			if !sf.IsSuppressed("LongMethod", "", lineOf(t, src, "val b = 2")) {
				t.Error("member @Suppress must cover the function body")
			}
			for _, probe := range []string{"val a = 1", "val c = 3", "val d = 4"} {
				if sf.IsSuppressed("LongMethod", "", lineOf(t, src, probe)) {
					t.Errorf("member @Suppress must not bleed to %q", probe)
				}
			}
		})
	}
}

func TestSuppressMisparsedClass_ClassLevel(t *testing.T) {
	for name, header := range misparsedHeaders {
		t.Run(name, func(t *testing.T) {
			src := misparsedClass("@Suppress(\"LargeClass\", \"TooManyFunctions\")\n", header)
			sf := BuildSuppressionFilter(parsedFileForFilter(t, "P.kt", src), nil, nil, "")
			for _, probe := range []string{"fun before", "val a = 1", "val b = 2", "fun plain", "val c = 3"} {
				if !sf.IsSuppressed("TooManyFunctions", "", lineOf(t, src, probe)) {
					t.Errorf("class-level @Suppress must cover %q", probe)
				}
			}
			for _, probe := range []string{"fun after", "val d = 4"} {
				if sf.IsSuppressed("TooManyFunctions", "", lineOf(t, src, probe)) {
					t.Errorf("class-level @Suppress must not cover code after the class (%q)", probe)
				}
			}
			if sf.IsSuppressed("MagicNumber", "", lineOf(t, src, "val b = 2")) {
				t.Error("unlisted rule must not be suppressed")
			}
			if !sf.IsSuppressed("LongMethod", "", lineOf(t, src, "val b = 2")) {
				t.Error("nested member @Suppress must still apply under a class-level one")
			}
		})
	}
}

func TestSuppressMisparsedClass_NonSuppressAnnotationsDoNotSuppress(t *testing.T) {
	src := misparsedClass("@Singleton\n@ContributesBinding(Scope::class)\n", misparsedHeaders["inject-newline"])
	sf := BuildSuppressionFilter(parsedFileForFilter(t, "P.kt", src), nil, nil, "")
	for _, rule := range []string{"LongMethod", "Scope::class", "MagicNumber"} {
		for _, probe := range []string{"fun before", "fun plain", "fun after", "val d = 4"} {
			if sf.IsSuppressed(rule, "", lineOf(t, src, probe)) {
				t.Errorf("%s must not be suppressed at %q", rule, probe)
			}
		}
	}
}

func TestSuppressMisparsedClass_BodylessClassDoesNotReachNextDeclaration(t *testing.T) {
	src := `@Suppress("LargeClass")
class Marker
private constructor()

fun next() {
  val x = 1
}
`
	sf := BuildSuppressionFilter(parsedFileForFilter(t, "M.kt", src), nil, nil, "")
	if sf.IsSuppressed("LargeClass", "", lineOf(t, src, "val x = 1")) {
		t.Error("class without a body must not suppress the next declaration")
	}
}

func TestSuppressMisparsedClass_BraceInStringAndComment(t *testing.T) {
	src := `@Suppress("LargeClass")
class Presenter
@Inject
constructor(private val a: Int) : Contract.Presenter {
  // an unbalanced { in a comment
  val s = "}"
  fun f() {
    val k = 1
  }
}

fun after() {
  val d = 4
}
`
	sf := BuildSuppressionFilter(parsedFileForFilter(t, "P.kt", src), nil, nil, "")
	if !sf.IsSuppressed("LargeClass", "", lineOf(t, src, "val k = 1")) {
		t.Error("braces in strings and comments must not end the class early")
	}
	if sf.IsSuppressed("LargeClass", "", lineOf(t, src, "val d = 4")) {
		t.Error("suppression must end with the class")
	}
}
