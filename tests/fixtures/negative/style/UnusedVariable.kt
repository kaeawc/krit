package fixtures.negative.style

fun foo() {
    val used = 1
    println(used)
}

class Bar {
    val classProperty = "not a local variable"

    companion object {
        const val DEFAULT_TIMEOUT_MS = 5_000
        val FALLBACK_NAME = "default"
    }
}

class Extension(objects: ObjectFactory) {
    public val enabled: Property<Boolean> = objects.property(Boolean::class.java)
}

fun interface Strategy {
    fun apply(): Boolean

    companion object {
        @JvmField val IGNORE = Strategy { false }
    }
}

enum class Direction(val degrees: Int) {
    NORTH(0),
    SOUTH(180);

    val radians: Double get() = degrees * 3.14159 / 180.0
}

fun walkPairs(pairs: List<Pair<Int, Int>>) {
    for ((a, b) in pairs) {
        println("a=$a b=${b}")
    }
}

fun walkPairsIgnoringFirst(pairs: List<Pair<Int, Int>>) {
    for ((_, b) in pairs) {
        println(b)
    }
}

// A unary-prefixed continuation (`+expr`) on the line after a declaration is
// folded by tree-sitter into the declaration's initializer. The variable is
// genuinely used inside that continuation, so it must NOT be reported.
fun unaryContinuationBareName(builder: Builder) {
    val factoryCall = builder.create()
    +factoryCall
}

fun unaryContinuationCallArgument(builder: Builder) {
    val typeKey = builder.key()
    +builder.add(typeKey)
}

fun unaryContinuationLambdaBody(builder: Builder) {
    val instance = builder.create()
    +builder.apply { instance.register() }
}

class Builder {
    fun create(): Builder = this
    fun key(): Int = 0
    fun add(k: Int): Builder = this
    fun register() {}
    operator fun Builder.unaryPlus() {}
}
