package fixtures.positive.potentialbugs

class EqualsAlwaysReturnsTrueOrFalse {
    override fun equals(other: Any?) = true

    override fun hashCode(): Int = 42
}
