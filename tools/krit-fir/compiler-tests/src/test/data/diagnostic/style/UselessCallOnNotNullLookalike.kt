// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23
// Lookalikes: the same names on other declarations are not the stdlib calls.
package test

class Wrapper(val value: String) {
    fun orEmpty(): String = value
    fun isNullOrEmpty(): Boolean = value.isEmpty()
}

fun member(w: Wrapper): String = w.orEmpty()

fun memberCheck(w: Wrapper): Boolean = w.isNullOrEmpty()

class Token

fun Token.orEmpty(): Token = this

fun extension(t: Token): Token = t.orEmpty()

fun listOfNotNull(vararg values: Token): List<Token> = values.toList()

fun localListOfNotNull(t: Token): List<Token> = listOfNotNull(t)
