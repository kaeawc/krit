package fixtures.positive.potentialbugs

class WrongEqualsTypeParameter(val id: Int) {
    override fun equals(other: String): Boolean {
        return id.toString() == other
    }
}
