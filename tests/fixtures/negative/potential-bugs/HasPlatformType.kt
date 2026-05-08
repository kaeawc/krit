package fixtures.negative.potentialbugs

class HasPlatformType {
    fun getLength(str: String): Int = java.lang.String(str).length()

    // Pure Kotlin expressions should not be flagged without resolver
    fun fromCode(code: Int) = entries.first { it.code == code }
    fun getCurrentAvatar() = getState().currentAvatar
    fun count() = items.size
}
