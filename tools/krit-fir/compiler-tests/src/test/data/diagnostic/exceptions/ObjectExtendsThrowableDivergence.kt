// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 20, 24, 28, 33, 40, 56
// Go findings FIR drops. Go takes every type name written anywhere in an
// object's delegation specifiers and matches Throwable, Exception, Error, or
// RuntimeException by simple name. These objects only mention an exception
// type; none of them is a Throwable.
package test

abstract class Holder(val value: Any)

// Go reports this because `Exception` is a type argument of the supertype;
// FIR is correct to drop it because the object is a Comparator, not a
// Throwable.
object ExceptionOrder : Comparator<Exception> {
    override fun compare(a: Exception, b: Exception): Int = 0
}

// Go reports this because `Exception` appears in a constructor argument;
// FIR is correct to drop it because the object extends Holder.
object ClassReferenceHolder : Holder(Exception::class)

// Go reports this because `Error` is a type argument inside a constructor
// argument; FIR is correct to drop it because the object extends Holder.
object TypeArgumentHolder : Holder(listOf<Error>())

// Go reports this because `Error` is a lambda parameter type in a constructor
// argument; FIR is correct to drop it because the object extends Holder.
object LambdaHolder : Holder({ e: Error -> e.message })

// Go reports this because `RuntimeException` is a type argument of an
// interface supertype; FIR is correct to drop it because the object is a
// function, not a Throwable.
object Wrapper : (String) -> RuntimeException {
    override fun invoke(message: String): RuntimeException = RuntimeException(message)
}

// Go reports this because it looks the object up by simple name, and the
// nested class below with the same name replaces it, so Go reads that class's
// supertypes; FIR is correct to drop it because this object is a Comparator.
object Clash : Comparator<Int> {
    override fun compare(a: Int, b: Int): Int = 0
}

class ClashHolder {
    class Clash : Exception()
}

interface Sink {
    fun put(t: Throwable)
}

// Go reports this because it reads every type name in the delegation
// specifiers, including `Throwable` and `Error` inside the `by` delegate
// expression; FIR is correct to drop it because the object is a Sink, not a
// Throwable.
object DelegatedErr : Sink by object : Sink {
    override fun put(t: Throwable) {
        val e: Error? = null
        if (e != null) throw e
    }
}
