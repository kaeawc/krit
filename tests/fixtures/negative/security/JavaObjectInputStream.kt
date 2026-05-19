package test

import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

class FilteringInputStream(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> {
        return super.resolveClass(desc)
    }
}

fun warn() {
    // String literal that merely mentions the FQN must not trip the rule.
    println("warning: never use java.io.ObjectInputStream(input) directly")
}
