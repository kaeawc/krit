// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 20, 24, 29, 32, 35, 40, 42, 46, 49x2, 53, 56, 59, 62, 65, 66, 71, 76, 80
// Operands the compiler types as nullable.
package test.unsafecall.positive

class Controller {
    fun start() {}
}

class Basic {
    private var controller: Controller? = null

    fun nextName(): String? = null

    fun member(nullable: String?) {
        val len = nullable<!UnsafeCallOnNullableType!>!!<!>.length
    }

    fun bare(nullable: String?) {
        val value = nullable<!UnsafeCallOnNullableType!>!!<!>
    }

    fun callResult(): Boolean {
        return nextName() != null && nextName()<!UnsafeCallOnNullableType!>!!<!>.isNotEmpty()
    }

    fun overwrittenWithNull() {
        controller = null
        controller<!UnsafeCallOnNullableType!>!!<!>.start()
    }

    fun safeCallChain(text: String?): Int = text?.trim()<!UnsafeCallOnNullableType!>!!<!>.length

    fun namedArgument(value: String?) {
        takes(text = value<!UnsafeCallOnNullableType!>!!<!>)
    }

    fun takes(text: String) {}

    fun lambdaParameter(items: List<String?>): List<String> = items.map { it<!UnsafeCallOnNullableType!>!!<!> }

    fun <T> unbounded(value: T): T = value<!UnsafeCallOnNullableType!>!!<!>

    // The `requireX` exemption is for expression bodies only.
    fun requireBlock(name: String?): String {
        return name<!UnsafeCallOnNullableType!>!!<!>
    }

    fun twoOnALine(a: String?, b: String?): Int = a<!UnsafeCallOnNullableType!>!!<!>.length + b<!UnsafeCallOnNullableType!>!!<!>.length
}

class Scopes(private val maybe: String?, private val other: Controller?) {
    val length: Int = maybe<!UnsafeCallOnNullableType!>!!<!>.length

    val label: String
        get() = "value ${maybe<!UnsafeCallOnNullableType!>!!<!>}"

    init {
        other<!UnsafeCallOnNullableType!>!!<!>.start()
    }

    fun parenthesized(): Int = (maybe)<!UnsafeCallOnNullableType!>!!<!>.length

    fun nested(first: Controller?, second: String?): Int {
        first<!UnsafeCallOnNullableType!>!!<!>.start()
        return second<!UnsafeCallOnNullableType!>!!<!>.length
    }

    fun anonymousObject(text: String?): Runnable = object : Runnable {
        override fun run() {
            text<!UnsafeCallOnNullableType!>!!<!>.length
        }
    }

    fun localFunction(text: String?): Int {
        fun inner() = text<!UnsafeCallOnNullableType!>!!<!>.length
        return inner()
    }

    fun inWhen(text: String?): Int = when (text<!UnsafeCallOnNullableType!>!!<!>) {
        "a" -> 1
        else -> 2
    }
}

