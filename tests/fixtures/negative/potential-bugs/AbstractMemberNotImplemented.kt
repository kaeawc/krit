package fixtures.negative.potentialbugs

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

abstract class Base {
    abstract fun handle()
}

interface Named {
    val name: String
}

// All members implemented in body.
class FullGreeter : Greeter {
    override fun greet(name: String): String = "Hi $name"
    override fun farewell() {}
}

// `name` provided via primary-constructor `override val`.
class NamedThing(override val name: String) : Named

// Abstract class doesn't have to implement anything.
abstract class StillAbstract : Greeter

// No supertypes — silent.
class Standalone {
    fun foo() = 42
}

// Concrete class extending abstract Base, implements abstract method.
class ImplBase : Base() {
    override fun handle() {}
}
