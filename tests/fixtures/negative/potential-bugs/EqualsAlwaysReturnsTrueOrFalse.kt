package fixtures.negative.potentialbugs

class EqualsAlwaysReturnsTrueOrFalse(val id: Int) {
    override fun equals(other: Any?) = this.id == (other as? EqualsAlwaysReturnsTrueOrFalse)?.id

    override fun hashCode(): Int = id.hashCode()
}
