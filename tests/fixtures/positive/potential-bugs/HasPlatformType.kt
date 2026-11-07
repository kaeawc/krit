package fixtures.positive.potentialbugs

class HasPlatformType {
    fun getName() = java.io.File("a").name
}
