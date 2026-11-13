// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every declaration is read, or exempt.
package test.unusedprivatepropertynegative

import kotlin.reflect.KProperty

annotation class Inject
annotation class Mock

private val topLevel = 1
private const val LIMIT = 2

fun readTop() = topLevel + LIMIT

class Reads(private val ctorVal: Int, plain: Int) {
    private val derived = plain + 1
    private val template = "value $ctorVal"
    private var counter = 0
    private val delegated by lazy { derived }
    private val referenced = 5
    private val inLambda = 6
    private val inDefault = 7
    private val ref = 8
    private val onlyInInit: Int
    private val inWhen = 10
    private val thisQualified = 11

    init {
        onlyInInit = 3
    }

    fun use(extra: Int = inDefault): Int {
        counter++
        val callable = ::referenced
        val bound = this::ref
        val lambda = { inLambda }
        val branch = when (extra) {
            inWhen -> 1
            else -> 2
        }
        return derived + template.length + delegated + extra + callable.get() + bound.get() + lambda() + branch + thisQualified + this.thisQualified
    }
}

class Substituted<T>(private val item: T) {
    fun peek(other: Substituted<String>): T = item
}

class Reader<T>(private val value: T) {
    fun read(other: Reader<String>): String = other.value
}

class Companions {
    private val fromCompanion = 1

    companion object {
        private const val SHARED = 2
        fun make(owner: Companions) = owner.fromCompanion + SHARED
    }
}

class Outer {
    private val seenByNested = 1

    class Nested(private val outer: Outer) {
        fun read() = outer.seenByNested
    }

    inner class Deep {
        fun read() = seenByNested
    }
}

val Outer.extra: Int get() = 4

private val String.shout: String get() = this + "!"

fun callShout() = "a".shout

object Registry {
    private val entries = mutableListOf<String>()
    fun add(entry: String) {
        entries += entry
    }
}

class Binding(private val owner: Any) {
    private operator fun getValue(thisRef: Any?, property: KProperty<*>): Int = 1
    private val viaDelegate by this
    fun go() = viaDelegate + owner.hashCode()
}

// Exempt by annotation, name or initializer.
class Exempt {
    @Inject private val injected: String = ""
    @Mock private lateinit var mocked: String
    @Suppress("unused") private val suppressed = 1
    @Suppress("UnusedPrivateProperty") private val suppressedByName = 2
    private val ignored = 3
    private val expected = 4
    private val serialVersionUID = 5L
}

class Owner {
    private val registered = register { 1 }
    private val launched = launch { 2 }
    private val observed = observeThing(1)
    private val chained = Owner().addChangeListener { }

    private fun register(block: () -> Int) = block()
    private fun launch(block: () -> Int) = block()
    private fun observeThing(value: Int) = value
    private fun addChangeListener(block: () -> Unit) = block()
}

class Tagged {
    companion object {
        private val TAG = tag()
        private fun tag() = "tag"
    }
}

data class Point(private val x: Int, private val y: Int)

@JvmInline
value class Wrapper(private val raw: Int)

annotation class Marker(val name: String, val count: Int = 0)

class UsedPlain(val exposed: Int, plain: Int) {
    val sum = exposed + plain
}

class UsedInSuper(arg: Int) : Parent(arg)

open class Parent(val value: Int)

class UsedInInit(config: String) {
    val upper: String

    init {
        upper = config.uppercase()
    }
}

// Delegation and annotation arguments read a declaration without a plain access.
interface Source {
    fun get(): Int
}

class Delegating(private val source: Source) : Source by source

class DelegatingParam(source: Source) : Source by source

class DelegatedProperty(shared: Lazy<Int>) {
    private val state = lazy { 1 }
    val a by state
    val b by shared
}

@Target(AnnotationTarget.CLASS)
annotation class Limit(val max: Int)

private const val MAX = 4

@Limit(MAX)
class Limited

class ConstructorDefault(private val base: Int, other: Int = base) {
    val total = other
}

class IsInitialized {
    private lateinit var late: String
    fun ready() = ::late.isInitialized
}

class WithReceiver {
    private val label = "l"
    fun render(builder: StringBuilder) = with(builder) { append(label) }
}

class Counter {
    private var hits = 0
    fun hit() = hits++
}

class Compound {
    private var total = 0
    fun add(n: Int) {
        total += n
    }
}

enum class Mode(private val code: Int) {
    ON(1),
    OFF(0);

    fun encode() = code
}
