package fixtures.positive.potentialbugs

class EqualsWithHashCodeExist(val id: Int) {
    override fun equals(other: Any?): Boolean {
        return this.id == (other as? EqualsWithHashCodeExist)?.id
    }
}
