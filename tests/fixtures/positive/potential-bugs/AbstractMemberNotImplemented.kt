package fixtures.positive.potentialbugs

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

abstract class Base {
    abstract fun handle()
}

// Missing greet AND farewell from Greeter.
class HollowGreeter : Greeter

// Implements greet but not farewell.
class HalfGreeter : Greeter {
    override fun greet(name: String): String = "Hi $name"
}

// Concrete class extends abstract Base without implementing handle.
class HollowConcrete : Base()
