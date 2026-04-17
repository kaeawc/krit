package style

fun processName(x: String?) {
    x?.let { println(it) }
}

fun handleValue(y: String?) {
    val result = y ?: "default"
    println(result)
}

// override function — parameter types dictated by superclass
open class Base {
    open fun process(x: String?) {
        println(x)
    }
}
class Derived : Base() {
    override fun process(x: String?) {
        println(x!!)
    }
}

// var reassigned to null
class Reassigned {
    private var value: String? = "hello"
    fun clear() {
        value = null
    }
}

// delegated property
class Delegated {
    val name: String? by lazy { "hello" }
}

// no initializer
class NoInit {
    val name: String?
        get() = "hello"
}

// mixed usage: some bang-bang, some safe-call
fun mixedUsage(x: String?) {
    println(x!!)
    x?.let { println(it) }
}

// lambda captures property and assigns null
class LambdaCaptured {
    private var value: String? = "hello"
    fun process(items: List<String>) {
        items.forEach { item ->
            if (item == "clear") {
                value = null
            }
        }
    }
}
