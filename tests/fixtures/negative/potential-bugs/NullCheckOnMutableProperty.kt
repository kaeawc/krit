package fixtures.negative.potentialbugs

class NullCheckOnMutableProperty {
    var mutableVar: String? = null

    fun check() {
        val local = mutableVar
        if (local != null) {
            println(local.length)
        }
    }
}
