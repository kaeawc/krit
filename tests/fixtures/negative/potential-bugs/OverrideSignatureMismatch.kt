package fixtures.negative.potentialbugs

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

class CorrectGreeter : Greeter {
    override fun greet(name: String): String = "Hi $name"
    override fun farewell() {}
}

// override of a library/framework member: the supertype isn't visible
// to the source resolver, so the rule should stay silent.
abstract class Logger
class MyLogger : Logger() {
    override fun toString(): String = "MyLogger"
}

// override with multiple-arity overloads on supertype — at least one matches.
abstract class Multi {
    abstract fun handle()
    abstract fun handle(x: Int)
}

class HandleImpl : Multi() {
    override fun handle() {}
    override fun handle(x: Int) {}
}
