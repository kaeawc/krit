package scanner

import "testing"

// A suppression on a constructor must cover a finding anchored on the class name.
func TestSuppressConstructorCoversClassNameAnchor(t *testing.T) {
	shapes := map[string]string{
		"newline header, annotations above constructor": `@Singleton
class Alpha
@Suppress("LongParameterList")
@JvmOverloads
constructor(
  private val a: Int,
  private val b: Int,
) : Base() {
  fun sibling() = 1
}

class Other(val x: Int)
`,
		"newline header, inject constructor": `class Alpha
@Suppress("LongParameterList")
@Inject
internal constructor(
  private val a: Int,
  private val b: Int,
) {
  fun sibling() = 1
}

class Other(val x: Int)
`,
		"same line header": `class Alpha @Suppress("LongParameterList") @Inject constructor(
  private val a: Int,
  private val b: Int,
) {
  fun sibling() = 1
}

class Other(val x: Int)
`,
	}
	for name, src := range shapes {
		t.Run(name, func(t *testing.T) {
			checkSuppressCase(t, src, suppressCase{
				rule:       "LongParameterList",
				suppressed: []string{"class Alpha"},
				reported:   []string{"fun sibling", "class Other"},
			})
		})
	}
}
