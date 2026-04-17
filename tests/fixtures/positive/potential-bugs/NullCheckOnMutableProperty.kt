package fixtures.positive.potentialbugs

class NullCheckOnMutableProperty {
    var mutableVar: String? = null

    fun check() {
        if (mutableVar != null) {
            println(mutableVar.length)
        }
    }
}
