// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 23, 31
// Local lookalikes. Go reports the zero-argument toLowerCase / toUpperCase calls
// and `String.format` by name; FIR does not, because each resolves to a project
// declaration, not to a JDK or stdlib API that uses the default locale. A call
// with an argument (`name.toUpperCase("!")`) is never the zero-argument
// default-locale conversion, so Go skips it too.
package test

class Name(private val value: String) {
    fun toLowerCase(): String = value.lowercase()

    fun toUpperCase(suffix: String): String = value.uppercase() + suffix
}

fun toLowerCase(): String = "local"

class Uses {
    fun lower(name: Name): String = name.toLowerCase()

    fun upper(name: Name): String = name.toUpperCase("!")

    fun topLevel(): String = toLowerCase()
}

class NestedStringObject {
    object String {
        fun format(pattern: kotlin.String, vararg args: Any?): kotlin.String = pattern
    }

    fun use(): kotlin.String = String.format("%d", 1)
}
