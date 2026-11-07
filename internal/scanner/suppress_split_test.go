package scanner

import "testing"

// splitBodyClass wraps members in a class whose header makes the grammar parse
// the body as a lambda, where member annotations become separate statements.
func splitBodyClass(members string) string {
	return `class Widget
@JvmOverloads
constructor(a: Int, b: Thing? = null, c: Int = 0) :
  Base(a, b, c) {
` + members + `
  fun sibling(p1: Int, p2: Int) {
    val s = 7
  }
}

fun after() {
  val d = 8
}
`
}

type suppressCase struct {
	rule       string
	suppressed []string // line needles the rule must be suppressed on
	reported   []string // line needles the rule must still be reported on
}

func checkSuppressCase(t *testing.T, src string, c suppressCase) {
	t.Helper()
	sf := BuildSuppressionFilter(parsedFileForFilter(t, "W.kt", src), nil, nil, "")
	for _, needle := range c.suppressed {
		if !sf.IsSuppressedAtColumn(c.rule, "", lineOf(t, src, needle), 1) {
			t.Errorf("%s must be suppressed on %q", c.rule, needle)
		}
	}
	for _, needle := range c.reported {
		if sf.IsSuppressedAtColumn(c.rule, "", lineOf(t, src, needle), 1) {
			t.Errorf("%s must not be suppressed on %q", c.rule, needle)
		}
	}
}

// requireSplitAnnotation fails the test unless the grammar parsed the
// annotation on the needle line without its argument list, the shape these
// tests cover.
func requireSplitAnnotation(t *testing.T, src, needle string) {
	t.Helper()
	f := parsedFileForFilter(t, "W.kt", src)
	line := lineOf(t, src, needle)
	for i := range f.FlatTree.Types {
		idx := uint32(i)
		if nodeTypeName(f.FlatTree.Types[idx]) == "annotation" && int(f.FlatTree.StartRows[idx])+1 == line {
			if flatAnnotationHasArgs(f.FlatTree, idx) {
				t.Fatalf("grammar now keeps the arguments of %q; this test no longer covers a split annotation", needle)
			}
			return
		}
	}
	t.Fatalf("no annotation node on line %d", line)
}

var splitBodyNegatives = []string{"fun sibling", "val s = 7", "fun after", "val d = 8"}

func TestSuppressSplitAnnotation_DetachedArgsInBody(t *testing.T) {
	src := splitBodyClass(`  init {
    x = 1
  }

  @Suppress("LongParameterList")
  fun show(
    p1: Int,
    p2: Int,
  ) {
    val v = 42
  }
`)
	requireSplitAnnotation(t, src, `@Suppress("LongParameterList")`)
	checkSuppressCase(t, src, suppressCase{
		rule:       "LongParameterList",
		suppressed: []string{`@Suppress("LongParameterList")`, "fun show(", "p2: Int,", "val v = 42"},
		reported:   splitBodyNegatives,
	})
}

func TestSuppressSplitAnnotation_DetachedArgsFollowedByComment(t *testing.T) {
	// Recovery from the dangling argument before it leaves the annotation, a
	// comment and the function as sibling nodes.
	src := `  companion object {
        confirm = confirm,
    @Suppress("FunctionNaming") // reason
    fun Factory(
      position: () -> Float,
      )
}

fun after() {
  val d = 8
}
`
	requireSplitAnnotation(t, src, `@Suppress("FunctionNaming")`)
	checkSuppressCase(t, src, suppressCase{
		rule:       "FunctionNaming",
		suppressed: []string{"fun Factory", "position: () -> Float"},
		reported:   []string{"confirm = confirm", "fun after", "val d = 8"},
	})
}

func TestSuppressSplitAnnotation_DetachedArgsAfterOtherAnnotation(t *testing.T) {
	src := splitBodyClass(`  init {
    x = 1
  }

  @JvmStatic
  @Suppress("LongParameterList")
  fun show(p1: Int) {
    val v = 42
  }
`)
	requireSplitAnnotation(t, src, `@Suppress("LongParameterList")`)
	checkSuppressCase(t, src, suppressCase{
		rule:       "LongParameterList",
		suppressed: []string{"fun show", "val v = 42"},
		reported:   splitBodyNegatives,
	})
}

func TestSuppressSplitAnnotation_DetachedArgsAsSiblingStatement(t *testing.T) {
	// A stray header before the body makes the annotation and its argument
	// list separate statements of a lambda.
	src := `) : Contract {
  @Suppress("LongMethod")
  override fun create(
                  enabled = id != null,
                )
}

fun after() {
  val d = 8
}
`
	requireSplitAnnotation(t, src, `@Suppress("LongMethod")`)
	checkSuppressCase(t, src, suppressCase{
		rule:       "LongMethod",
		suppressed: []string{"override fun create", "enabled = id != null"},
		reported:   []string{"fun after", "val d = 8"},
	})
}

func TestSuppressSplitAnnotation_DetachedArgsWithoutFollowingStatement(t *testing.T) {
	src := splitBodyClass(`  init {
    x = 1
  }
`) + "\n@Suppress(\"LongMethod\")\n"
	checkSuppressCase(t, src, suppressCase{
		rule:     "LongMethod",
		reported: append([]string{"x = 1"}, splitBodyNegatives...),
	})
}

func TestSuppressSplitAnnotation_CommentBetweenClassAnnotations(t *testing.T) {
	src := `@Suppress("LargeClass")
// a note about the next annotation
@OptIn(Experimental::class)
class Screens : Base(Level.Low) {
  fun one() {
    val v = 42
  }
}

class Other {
  fun two() {
    val w = 43
  }
}
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "LargeClass",
		suppressed: []string{"class Screens", "fun one", "val v = 42"},
		reported:   []string{"class Other", "val w = 43"},
	})
}

func TestSuppressSplitAnnotation_ParameterAnnotationAtLineStart(t *testing.T) {
	src := `class Repo(private val r: Int) {
  suspend fun submit(
    id: String,
    @Suppress("UnusedParameter") token: String,
    state: Int,
  ): Int = 1
}
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "UnusedParameter",
		suppressed: []string{"token: String"},
		reported:   []string{"id: String", "state: Int", "suspend fun submit"},
	})
}

func TestSuppressPropertyAccessors(t *testing.T) {
	src := `class Holder(val items: List<Int>) {
  @Suppress("MagicNumber") // reason
  var count: Int
    get() {
      return items.size + 42
    }
    set(value) {
      store(value * 43)
    }

  val other: Int
    get() = items.size + 44
}
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "MagicNumber",
		suppressed: []string{"var count", "items.size + 42", "store(value * 43)"},
		reported:   []string{"val other", "items.size + 44"},
	})
}

func TestSuppressPropertyAccessorsAtTopLevel(t *testing.T) {
	src := `@Suppress("MagicNumber")
val count: Int
  get() = 42

val other: Int
  get() = 43
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "MagicNumber",
		suppressed: []string{"get() = 42"},
		reported:   []string{"val other", "get() = 43"},
	})
}

var detachedConstructorClassHeaders = map[string]string{
	"data-class": `@Suppress("FunctionNaming")
data class Payload
internal constructor(
  @get:JvmName("first") val first: String,
  val second: List<String>,
) {
`,
	"supertypes-on-own-lines": `@Suppress("FunctionNaming")
@Marker(Scope::class)
class Payload
constructor(
  a: Store,
  private val b: Int,
) :
  Base(a),
  Api {
`,
}

func TestSuppressClassWithDetachedConstructorParsedAsExpression(t *testing.T) {
	for name, header := range detachedConstructorClassHeaders {
		t.Run(name, func(t *testing.T) {
			src := header + `  fun read_value(x: Int): Int = x

  class Builder {
    fun set_value(x: Int) = x
  }
}

fun after_payload() = 1
`
			checkSuppressCase(t, src, suppressCase{
				rule:       "FunctionNaming",
				suppressed: []string{"fun read_value", "fun set_value"},
				reported:   []string{"fun after_payload"},
			})
		})
	}
}
