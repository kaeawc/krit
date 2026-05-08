package complexity

class MethodOverloading {
    fun process() {}
    fun process(a: Int) {}
    fun process(a: String) {}
    fun process(a: Int, b: Int) {}
    fun process(a: Int, b: String) {}
    fun process(a: String, b: String) {}
    fun process(a: Int, b: Int, c: Int) {}
}
