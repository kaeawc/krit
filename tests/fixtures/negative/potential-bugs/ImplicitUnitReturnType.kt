package fixtures.negative.potentialbugs

class ImplicitUnitReturnType {
    fun foo(items: MutableList<Int>): Unit = items.clear()

    fun bar(): String {
        return "hello"
    }

    fun baz(items: MutableList<Int>) {
        items.clear()
    }
}
