package dev.jasonpearson.krit.fir.checkers.androidlint

internal object PendingIntentFlagOperators {
    fun unary(name: String, value: Int): Int? = when (name) {
        "unaryMinus" -> -value
        "unaryPlus" -> value
        "inv" -> value.inv()
        else -> null
    }

    fun binary(name: String, left: Int, right: Int): Int? = when (name) {
        "or" -> left or right
        "and" -> left and right
        "xor" -> left xor right
        "shl" -> left shl right
        "shr" -> left shr right
        "ushr" -> left ushr right
        else -> arithmetic(name, left, right)
    }

    private fun arithmetic(name: String, left: Int, right: Int): Int? = when (name) {
        "plus" -> left + right
        "minus" -> left - right
        "times" -> left * right
        "div" -> if (right == 0) null else left / right
        "rem" -> if (right == 0) null else left % right
        else -> null
    }
}
