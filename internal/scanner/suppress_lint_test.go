package scanner

import "testing"

func TestSuppressLintAnnotation(t *testing.T) {
	src := `import android.annotation.SuppressLint

class Widget {
    @SuppressLint("SdCardPath")
    fun one() = "one"

    @SuppressLint("Other", "SdCardPath")
    fun two() = "two"

    @SuppressLint(value = ["SdCardPath"])
    fun three() = "three"

    @Suppress(names = ["SdCardPath"])
    fun four() = "four"

    @SuppressLint("Other")
    fun five() = "five"

    fun six() = "six"
}

@SuppressLint("all")
class Everything {
    fun seven() = "seven"
}
`
	checkSuppressCase(t, src, suppressCase{
		rule:       "SdCardPath",
		suppressed: []string{`fun one`, `fun two`, `fun three`, `fun four`, `fun seven`},
		reported:   []string{`fun five`, `fun six`},
	})
}
