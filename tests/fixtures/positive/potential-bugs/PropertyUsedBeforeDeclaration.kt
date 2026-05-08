package fixtures.positive.potentialbugs

class PropertyUsedBeforeDeclaration {
    val first = second
    val second = 42
}
