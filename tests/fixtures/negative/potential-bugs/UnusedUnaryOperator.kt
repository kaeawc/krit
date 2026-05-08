package fixtures.negative.potentialbugs

class UnusedUnaryOperator {
    fun assigned(x: Int): Int {
        val y = +x
        return y
    }

    fun negativeAssigned(x: Int): Int {
        val y = -x
        return y
    }

    fun returnedDirectly(x: Int): Int {
        return -x
    }

    fun usedInParens() {
        val x = (1 + 2
            + 3 + 4 + 5)
    }

    fun usedAsArgument(x: Int) {
        println(-x)
    }

    fun usedInAnnotation() {
        val x = -1
    }

    fun negation() {
        val flag = true
        val result = !flag
    }

    fun inString() {
        val s = "+x is a string"
        val t = "-y is also a string"
    }

    fun inIfExpression(b: Boolean) {
        var x = 0
        x = if (b) {
            -1
        } else {
            1
        }
    }

    fun inTryCatchExpression(): Int {
        val code: Int = try {
            something()
        } catch (e: Exception) {
            -1
        }
        return code
    }

    fun inTryBlockExpression(): Int {
        return try {
            -1
        } catch (e: Exception) {
            0
        }
    }

    fun inFinallyExpression(): Int {
        val x: Int = try {
            something()
        } finally {
            -1
        }
        return x
    }
}
