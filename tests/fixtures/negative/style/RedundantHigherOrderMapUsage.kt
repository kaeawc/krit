package style

data class Person(val name: String)

fun example() {
    val people = listOf(Person("Alice"), Person("Bob"))
    val names = people.map { it.name }
}
