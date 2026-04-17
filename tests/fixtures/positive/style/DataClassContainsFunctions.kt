package style

data class User(val name: String, val age: Int) {
    fun greet() = "Hello, $name"

    fun compute() = age * 2
}
