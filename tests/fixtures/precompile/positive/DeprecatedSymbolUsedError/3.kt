// EXPECTED-KOTLINC-ERROR: DEPRECATION_ERROR
class Container {
    @Deprecated("Use newCount", level = DeprecationLevel.ERROR)
    val oldCount: Int = 0

    fun read(other: Container): Int {
        return other.oldCount
    }
}
