// fir-parity: skip K2 rejects kotlin.synchronized on a primitive lock (SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE is an error from language version 2.1), so this fixture never compiles cleanly and Go stays authoritative for it; the FIR positives are covered by compiler-tests data through a monitor-lock wrapper
package test

class Counter {
    val count: Int = 1

    fun work() {
        synchronized(count) {
            println("work")
        }
    }
}
