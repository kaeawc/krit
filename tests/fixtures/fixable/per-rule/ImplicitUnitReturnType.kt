package fixtures.positive.potentialbugs

class ImplicitUnitReturnType {
    fun foo(items: MutableList<Int>) = items.clear()
}
