package fixtures.negative.potentialbugs

class UselessPostfixExpression {
    fun example(x: Int): Int {
        var value = x
        value++
        return value
    }
}
