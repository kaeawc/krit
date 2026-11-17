// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Objects that only mention an exception type in their delegation specifiers.
// None of them is a Throwable, so neither Go nor FIR reports them: Go reads only
// the superclass, the specifier that calls a constructor, by its own name.
package test

abstract class Holder(val value: Any)

// `Exception` is a type argument; the object is a Comparator.
object ExceptionOrder : Comparator<Exception> {
    override fun compare(a: Exception, b: Exception): Int = 0
}

// `Exception` appears in a constructor argument; the object extends Holder.
object ClassReferenceHolder : Holder(Exception::class)

// `Error` is a type argument inside a constructor argument; the object
// extends Holder.
object TypeArgumentHolder : Holder(listOf<Error>())

// `Error` is a lambda parameter type in a constructor argument; the object
// extends Holder.
object LambdaHolder : Holder({ e: Error -> e.message })

// `RuntimeException` is a type argument of an interface supertype; the
// object is a function.
object Wrapper : (String) -> RuntimeException {
    override fun invoke(message: String): RuntimeException = RuntimeException(message)
}

// A nested class below with the same name extends Exception; this object is
// a Comparator.
object Clash : Comparator<Int> {
    override fun compare(a: Int, b: Int): Int = 0
}

class ClashHolder {
    class Clash : Exception()
}

interface Sink {
    fun put(t: Throwable)
}

// `Throwable` and `Error` appear inside the `by` delegate expression; the
// object is a Sink.
object DelegatedErr : Sink by object : Sink {
    override fun put(t: Throwable) {
        val e: Error? = null
        if (e != null) throw e
    }
}
