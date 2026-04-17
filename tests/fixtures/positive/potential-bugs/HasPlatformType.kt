package fixtures.positive.potentialbugs

class HasPlatformType {
    fun getLength(str: String) = java.lang.String(str).length()
}
