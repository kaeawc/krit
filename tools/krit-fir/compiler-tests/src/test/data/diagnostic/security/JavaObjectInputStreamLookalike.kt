// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: a call spelled ObjectInputStream(...) that does not create a
// java.io.ObjectInputStream. This comment mentions java.io.ObjectInputStream,
// which is enough for Go's file gate.
package test

class ObjectInputStream(val source: String)

class Reader {
    // Go reports this because the call is spelled ObjectInputStream and the
    // file mentions java.io.ObjectInputStream; FIR is correct to drop it
    // because the constructed class is test.ObjectInputStream.
    fun open(): ObjectInputStream = ObjectInputStream("local")

    // The fully qualified JDK call still is one.
    fun jdk(input: java.io.InputStream): java.io.ObjectInputStream = <!JavaObjectInputStream!>java.io.ObjectInputStream(input)<!>
}
