package fixtures.positive.potentialbugs

class UselessPostfixExpression {
    fun example(x: Int): Int {
        var value = x
        return value++
    }
}
