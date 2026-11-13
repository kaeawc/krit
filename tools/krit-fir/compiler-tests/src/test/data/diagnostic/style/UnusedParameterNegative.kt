// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every parameter is read, or the function is exempt.
package test.unusedparameternegative

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

annotation class Subscribe
annotation class Provides
annotation class Test
annotation class Preview
annotation class ConstellationPreviews
annotation class Composable

interface Base {
    fun render(id: String)
    fun withBody(id: String) {
        println("interface default")
    }
}

class Impl : Base {
    override fun render(id: String) {
        println("framework override")
    }
}

open class Parent {
    open fun hook(event: String) {
        println("open")
    }

    protected fun guarded(event: String) {
        println("protected")
    }

    operator fun plus(other: Parent): Parent {
        return this
    }
}

abstract class Abstract {
    abstract fun absent(value: Int)
}

fun used(value: Int) {
    println(value)
}

fun lambdaCapture(id: String) {
    listOf(1).map { id.length + it }
}

fun nestedFunctionCapture(id: String) {
    fun nested() {
        println(id)
    }
    nested()
}

fun stringInterpolation(endpoint: String) {
    println("https://example.com/$endpoint")
}

fun templateWithBraces(endpoint: String) {
    println("https://example.com/${endpoint}")
}

fun laterDefaultArgument(oldState: String, value: String = oldState) {
    println(value)
}

fun invokesFunctionParameter(action: () -> Unit) {
    action()
}

fun invokesWithInvoke(action: (Int) -> Int) {
    println(action.invoke(1))
}

class Config

fun configure(init: Config.() -> Unit): Config {
    val configuration = Config()
    configuration.init()
    return configuration
}

fun callableReference(target: String) {
    val reference = target::length
    println(reference)
}

fun namedArgument(value: Int) {
    used(value = value)
}

fun forLoopIterableReference(params: List<String>) {
    for (param in params) {
        println(param)
    }
}

fun destructured(entries: Map<String, String>) {
    for ((k, v) in entries) {
        println("$k=$v")
    }
}

class KSAnnotation {
    fun process() {}
}

fun receivesSoftKeywordParam(annotation: KSAnnotation) {
    annotation.process()
}

fun placeholders(ignored: String, expected: Int) {
    println("allowed names")
}

fun explicitlySuppressedUpper(@Suppress("UNUSED_PARAMETER") targetInfo: KSAnnotation): Int {
    return 0
}

fun explicitlySuppressedLower(@Suppress("unused") leftover: KSAnnotation): Int {
    return 0
}

fun suppressedAmongUsed(
    @Suppress("UNUSED_PARAMETER") unusedFirst: KSAnnotation,
    second: KSAnnotation,
): Int {
    second.process()
    return 0
}

@Subscribe
fun subscribed(event: String) {
    println("framework entrypoint")
}

@Provides
fun provides(dependency: String) {
    println("dagger")
}

@Test
fun testCase(fixture: String) {
    println("runner")
}

@Preview
fun previewFunction(parameter: String) {
    println("preview")
}

@ConstellationPreviews
fun wrappedPreview(parameter: String) {
    println("wrapped preview")
}

@Composable
fun composable(content: String) {
    println("composable")
}

fun emptyBody(value: Int) {}

fun emptyBodySpaced(value: Int) { }

fun unitBody(value: Int) = Unit

fun todoBody(value: Int): Int {
    TODO("later")
}

fun todoExpression(value: Int): Int = TODO("later")

fun errorExpression(value: Int): Int = error("unsupported")

fun throwsBody(value: Int): Int {
    throw IllegalStateException("no")
}

fun throwsExpression(value: Int): Int = throw IllegalStateException("no")

fun commentOnly(value: Int) {
    // nothing yet
}

interface Listener {
    fun onStart()
    fun onEnd()
}

fun makeListener(onStart: () -> Unit, onEnd: () -> Unit): Listener {
    return object : Listener {
        override fun onStart() {
            onStart()
        }

        override fun onEnd() {
            onEnd()
        }
    }
}

fun recordTiming(group: String, durations: MutableMap<String, MutableList<Long>>) {
    durations.getOrPut(group) { mutableListOf(group.length.toLong()) }
}

fun usedInLocalClass(seed: Int) {
    class Local {
        fun get(): Int = seed
    }
    println(Local().get())
}

fun usedInObjectExpression(seed: Int): Any {
    return object {
        val value = seed
    }
}

fun usedAsReceiver(text: String): Int {
    return text.length
}

fun usedInWhen(kind: Int): String {
    return when (kind) {
        1 -> "one"
        else -> "other"
    }
}

fun usedInCondition(flag: Boolean): Int {
    if (flag) return 1
    return 0
}

fun assigned(count: Int): Int {
    var total = 0
    total += count
    return total
}

fun delegated(seed: Int) {
    val lazyValue by lazy { seed }
    println(lazyValue)
}

class Outer {
    fun member(flag: Boolean) {
        println(flag)
    }

    inner class Inner {
        fun nested(flag: Boolean) {
            println(flag)
        }
    }
}

interface WithCompanion {
    companion object {
        fun make(seed: Int): String {
            return "made"
        }
    }

    object Nested {
        fun build(seed: Int): String {
            return "built"
        }
    }
}

fun interface Handler {
    fun handle(event: String)
}

@OptIn(ExperimentalContracts::class)
fun contractUse(value: Any?, other: Int) {
    contract { returns() implies (value != null) }
    println(other)
}

open class TaggedBase(val tag: String)

fun superConstructorArgument(tag: String): Any {
    return object : TaggedBase(tag) {}
}

interface Greeter {
    fun greet(): String
}

fun delegatedInterface(greeter: Greeter): Greeter {
    return object : Greeter by greeter {}
}

fun smartCast(value: Any) {
    if (value is String) {
        println(value.length)
    }
}

fun safeCallLet(value: String?) {
    value?.let { println(it) }
}

fun propertyDelegate(source: Lazy<Int>) {
    val copy by source
    println(copy)
}

fun reflectionClass(value: Any) {
    println(value::class)
}

fun elvisUse(value: String?): String {
    return value ?: "none"
}

fun indexedUse(items: List<Int>): Int {
    return items[0]
}

fun comparisonUse(limit: Int): Boolean {
    return 3 > limit
}

fun varargUse(vararg names: String): Int {
    return names.size
}

fun spreadUse(names: Array<String>) {
    varargUse(*names)
}

fun lambdaInvokedInline(block: () -> Unit) {
    run { block() }
}

fun passedAlong(block: () -> Unit) {
    run(block)
}

fun reassignedCopy(count: Int): Int {
    var total = count
    total++
    return total
}
