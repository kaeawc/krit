package style

class Person(var name: String, var age: Int)

fun example() {
    val person = Person("Alice", 30).also { println(it) }
    person.also { log(it.toString()) }
}
