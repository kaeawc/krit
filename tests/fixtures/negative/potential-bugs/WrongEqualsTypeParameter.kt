package fixtures.negative.potentialbugs

class WrongEqualsTypeParameter(val id: Int) {
    override fun equals(other: Any?): Boolean {
        return id == (other as? WrongEqualsTypeParameter)?.id
    }

    override fun hashCode(): Int = id.hashCode()
}
