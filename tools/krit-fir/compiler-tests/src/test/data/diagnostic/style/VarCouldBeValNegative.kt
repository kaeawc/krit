// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every var is reassigned, or is not examined.
package test.varcouldbevalnegative

import kotlin.properties.Delegates

annotation class Inject

interface Named {
    var label: String
}

class Box(var x: Int)

fun reassigned() {
    var a = 1
    a = 2
    println(a)

    var b = 0
    b += 10

    var c = 5
    c++

    var d = 3
    --d

    var e: Int
    e = 7

    println(b + c + d + e)
}

fun capturedWrite() {
    var counter = 0
    listOf(1, 2).forEach { counter += it }
    println(counter)
}

fun delegatedLocal() {
    var observed by Delegates.observable(0) { _, _, _ -> }
    println(observed)
}

class Container : Named {
    // Not private: may be reassigned elsewhere.
    var publicProp = 1
    internal var internalProp = 1
    protected var protectedProp = 1

    // An override must stay a var.
    override var label = "x"

    // An explicit setter.
    private var withSetter: Int = 0
        set(value) { field = value }

    private var listener: (() -> Unit)? = null

    fun setListener(value: (() -> Unit)?) {
        this.listener = value
    }

    private var builderFlag = false

    fun enable(): Container {
        this.builderFlag = true
        return this
    }

    private var count = 0

    fun bump() {
        count++
    }

    private var viaOther = 0

    fun copyFrom(other: Container) {
        other.viaOther = 5
    }

    private var delegated by Delegates.notNull<Int>()

    fun use() = publicProp + internalProp + protectedProp + withSetter + delegated
}

class Injected {
    @Inject
    private var service: String? = null

    @Volatile
    private var ready = false

    @Suppress("unused")
    private var spare = 1

    fun read() = service to ready
}

class Constructed(private var x: Int, var y: Int) {
    fun read() = x + y
}

class Destructured {
    fun split() {
        var (first, second) = 1 to 2
        println(first + second)
    }
}

object LateinitHolder {
    private lateinit var provider: String

    fun init(value: String) {
        provider = value
    }

    fun read() = provider
}

class ProgressService {
    companion object {
        private var title: String = ""
    }

    class Controller {
        fun update(newTitle: String) {
            ProgressService.title = newTitle
        }
    }
}

object Deps {
    private var configured = false

    fun configure() {
        Deps.configured = true
    }

    fun read() = configured
}

class CompanionWrite {
    companion object {
        private var shared = 0
    }

    fun bump() {
        shared = shared + 1
    }
}

class Generic<T>(private val seed: T) {
    private var last: T? = null

    fun merge(other: Generic<T>) {
        other.last = seed
    }
}
