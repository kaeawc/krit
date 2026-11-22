// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 9, 10, 11, 12, 13, 16, 23, 25, 30, 33
package elvis.basics

class ElvisBasics(val label: String) {
    fun local(value: String): String {
        val text: String = "hello"
        val local = <!UselessElvisOnNonNull!>text ?: "fallback"<!>
        val parameter = <!UselessElvisOnNonNull!>value ?: "fallback"<!>
        val literal = <!UselessElvisOnNonNull!>"always" ?: "dead"<!>
        val integer = <!UselessElvisOnNonNull!>42 ?: 0<!>
        val boolean = <!UselessElvisOnNonNull!>true ?: false<!>
        val character = <!UselessElvisOnNonNull!>'a' ?: 'b'<!>
        // Go misses this constructor property; FIR resolves its declared non-null String type.
        val member = <!UselessElvisOnNonNull!>this.label ?: "dead"<!>
        val parenthesized = <!UselessElvisOnNonNull!>(value) ?: "dead"<!>
        return local + parameter + literal + integer + boolean + character + member + parenthesized
    }

    // Go misses the asserted nullable parameter; !! guarantees a non-null result if it completes.
    fun assertion(value: String?): String = <!UselessElvisOnNonNull!>value!! ?: "dead"<!>

    fun cast(value: Any): String = <!UselessElvisOnNonNull!>(value as String) ?: "dead"<!>

    fun multiline(value: String): String = <!UselessElvisOnNonNull!>value<!> ?:
        "dead"
}

object ElvisScope {
    val text = <!UselessElvisOnNonNull!>"object" ?: "dead"<!>
}

val topLevelElvis = <!UselessElvisOnNonNull!>"top" ?: "dead"<!>
