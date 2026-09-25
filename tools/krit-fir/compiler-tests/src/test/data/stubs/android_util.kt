// Compiler-test source stubs; never packaged in the production artifact.
package android.util

interface AttributeSet {
    val attributeCount: Int

    fun getAttributeValue(namespace: String?, name: String?): String?
}

// Java class of static logging methods; the tag is @Nullable in the SDK.
object Log {
    const val VERBOSE: Int = 2
    const val DEBUG: Int = 3
    const val INFO: Int = 4
    const val WARN: Int = 5
    const val ERROR: Int = 6
    const val ASSERT: Int = 7

    fun v(tag: String?, msg: String): Int = TODO()

    fun v(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun d(tag: String?, msg: String): Int = TODO()

    fun d(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun i(tag: String?, msg: String): Int = TODO()

    fun i(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun w(tag: String?, msg: String): Int = TODO()

    fun w(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun w(tag: String?, tr: Throwable?): Int = TODO()

    fun e(tag: String?, msg: String): Int = TODO()

    fun e(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun wtf(tag: String?, msg: String?): Int = TODO()

    fun wtf(tag: String?, tr: Throwable): Int = TODO()

    fun wtf(tag: String?, msg: String?, tr: Throwable?): Int = TODO()

    fun println(priority: Int, tag: String?, msg: String): Int = TODO()

    fun isLoggable(tag: String?, level: Int): Boolean = TODO()

    fun getStackTraceString(tr: Throwable?): String = TODO()
}

open class SparseArray<E> : Cloneable {
    constructor()

    constructor(initialCapacity: Int)

    // Java `E get(int)`: Kotlin treats Java get() as an operator; the Kotlin
    // stub must say so explicitly.
    operator fun get(key: Int): E? = TODO()

    fun get(key: Int, valueIfKeyNotFound: E): E = TODO()

    fun put(key: Int, value: E) {
        TODO()
    }

    fun remove(key: Int) {
        TODO()
    }

    fun size(): Int = TODO()

    fun keyAt(index: Int): Int = TODO()

    fun valueAt(index: Int): E = TODO()

    fun clear() {
        TODO()
    }
}

class Size(val width: Int, val height: Int)

class SizeF(val width: Float, val height: Float)

open class AndroidException : Exception {
    constructor()

    constructor(name: String?)

    constructor(name: String?, cause: Throwable?)
}
