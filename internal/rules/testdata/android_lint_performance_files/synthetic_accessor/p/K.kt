package p

class K {
    private var field = 0
    private val list = mutableListOf<Int>()
    private fun helper() {}
    private constructor(x: String)
    constructor()

    inner class Inner {
        private fun innerMethod() {}
        private val innerProp = 1
        fun use() {
            field++
            helper()
            list.add(1)
            K("x")
        }
    }

    private class PrivateNested {
        fun x() {}
    }

    class Nested {
        private fun nestedMethod() {}
        fun callOuter(k: K) { k.helper() }
    }

    fun outer() {
        val i = Inner()
        i.innerMethod()
        val v = i.innerProp
        PrivateNested()
        Nested().nestedMethod()
        val r = Runnable { helper() }
        val o = object : Runnable { override fun run() { helper() } }
    }

    companion object {
        private fun companionPrivate() {}
        fun fromCompanion(k: K) { k.helper() }
    }

    fun callCompanion() { companionPrivate() }
}

private fun topLevelPrivate() {}

class UsesTop {
    fun x() { topLevelPrivate() }
}

fun topCaller() { topLevelPrivate() }

sealed class S private constructor() {
    class A : S()
}

data class D private constructor(val x: Int) {
    class B { fun make() = D(1).copy(x = 2) }
}
