// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

typealias NullableName = String?

class Holder {
    var mutableProp: String? = null
}

class Negatives {
    fun safeCast(input: Any?) {
        val result = input as? String
        println(result)
    }

    fun nullableTarget(input: String?) {
        val result = input as String?
        println(result)
    }

    fun nullLiteral() {
        val result = null as String?
        println(result)
    }

    fun nonNullSource(nonNull: String) {
        val result = nonNull as CharSequence
        println(result)
    }

    fun aliasTarget(input: String?) {
        val result = input as NullableName
        println(result)
    }

    fun nullableBoundTarget(input: Any?): String? {
        return castTo<String?>(input)
    }

    @Suppress("UNCHECKED_CAST")
    fun <T> castTo(input: Any?): T = input as T

    fun commentsAndStrings(input: String?) {
        // input as String
        val text = "input as String"
        println(text)
        val safe = input as? String
        println(safe)
    }

    fun shadowing(input: String?) {
        run {
            val input: String = "local"
            val result = input as CharSequence
            println(result)
        }
    }

    fun smartCastFromNullCheck(input: String?) {
        if (input != null) {
            val result = input as CharSequence
            println(result)
        }
    }

    fun smartCastFromEarlyReturn(input: String?) {
        if (input == null) return
        val result = input as CharSequence
        println(result)
    }

    fun smartCastFromIs(input: Any?) {
        if (input is String) {
            val result = input as CharSequence
            println(result)
        }
    }

    fun smartCastFromRequire(input: String?) {
        requireNotNull(input)
        val result = input as CharSequence
        println(result)
    }

    fun notNullAssertion(input: String?) {
        val result = input!! as CharSequence
        println(result)
    }

    fun elvisToNonNull(input: String?) {
        val result = (input ?: "") as CharSequence
        println(result)
    }

    fun nestedCast(input: String?) {
        val result = (input as Any?) as String
        println(result)
    }

    fun nestedSafeCast(input: Any?) {
        val result = (input as? Any) as String
        println(result)
    }

    fun smartCastOnLocalVal(holder: Holder) {
        val copy = holder.mutableProp
        if (copy != null) {
            val result = copy as CharSequence
            println(result)
        }
    }

    fun platformType(): String {
        val builder = java.lang.StringBuilder("x")
        return builder.toString() as String
    }

    fun nullableBoundOperand(input: Any?): Any? = input as Any?

    fun nonNullAnyTarget(input: String): Any = input as Any
}
