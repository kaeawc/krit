package fixtures.negative.potentialbugs

class EqualsWithHashCodeExist(val id: Int) {
    override fun equals(other: Any?): Boolean {
        return this.id == (other as? EqualsWithHashCodeExist)?.id
    }

    override fun hashCode(): Int {
        return id.hashCode()
    }
}
