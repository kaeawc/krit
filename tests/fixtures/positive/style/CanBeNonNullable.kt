package style

class Example {
    val name: String? = "hello"
}

class VarExample {
    private var count: Int? = 0
    fun increment() {
        count = count!! + 1
    }
}

fun allBangBang(x: String?) {
    println(x!!)
    val y = x!!.length
}
