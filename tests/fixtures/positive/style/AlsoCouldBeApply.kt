package style

class Person(var name: String, var age: Int)

fun example() {
    val person = Person("Alice", 30).also { it.name = "Bob"; it.age = 25 }
}
