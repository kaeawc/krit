package fixtures.negative.potentialbugs

class PropertyUsedBeforeDeclaration {
    fun example() {
        val x = 1
        println(x)
    }
}
