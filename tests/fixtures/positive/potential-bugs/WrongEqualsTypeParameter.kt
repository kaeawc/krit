package fixtures.positive.potentialbugs

class WrongEqualsTypeParameter(val id: Int) {
    fun equals(other: String): Boolean {
        return id.toString() == other
    }
}
