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
