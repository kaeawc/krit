// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 20, 22, 30
// Local lookalikes. Go reports every call below by name (toLowerCase /
// toUpperCase on any receiver, format on the receiver text `String`); FIR
// does not, because each resolves to a project declaration, not to a JDK or
// stdlib API that uses the default locale.
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
