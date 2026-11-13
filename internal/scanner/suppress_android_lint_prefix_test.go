package scanner

import "testing"

// Android Lint accepts the Android Studio spelling AndroidLint<Id> in @SuppressLint and @Suppress.
func TestSuppressAndroidLintPrefixedID(t *testing.T) {
	src := `class Holder {
    fun local() {
        @SuppressLint("AndroidLintUseSparseArrays") val a = build()
    }

    @SuppressLint("AndroidLintUseSparseArrays")
    fun member() {
        val b = build()
    }

    @Suppress("AndroidLintOtherRule")
    fun other() {
        val c = build()
    }

    fun plain() {
        val d = build()
    }
}
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "UseSparseArrays",
		suppressed: []string{"val a = build", "val b = build"},
		reported:   []string{"val c = build", "val d = build"},
	})
}
