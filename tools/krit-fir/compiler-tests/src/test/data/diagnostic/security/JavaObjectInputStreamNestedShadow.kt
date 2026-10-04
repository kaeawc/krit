// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 23, 29, 32
// Negative: a nested class and a function named ObjectInputStream win over the
// java.io import inside their scope, so those calls do not create a
// java.io.ObjectInputStream.
package test

import java.io.InputStream
import java.io.ObjectInputStream

class Holder {
    class ObjectInputStream(val source: String)

    // Go reports this because the call is spelled ObjectInputStream and the
    // file imports java.io.ObjectInputStream; FIR is correct to drop it
    // because the constructed class is Holder.ObjectInputStream.
    fun open(): ObjectInputStream = ObjectInputStream("nested")
}

object Factory {
    fun ObjectInputStream(input: InputStream, tag: String): java.io.ObjectInputStream {
        println(tag)
        return <!JavaObjectInputStream!>java.io.ObjectInputStream(input)<!>
    }

    // Go reports this because the call is spelled ObjectInputStream; it calls
    // the factory above, not a constructor, and FIR reports the constructor
    // call inside the factory instead.
    fun open(input: InputStream): java.io.ObjectInputStream = ObjectInputStream(input, "tag")
}

fun outside(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
