package fixtures.positive.potentialbugs

@Deprecated("Use NewClass instead")
class OldClass

class Deprecation {
    @Deprecated("Use newMethod instead")
    fun oldMethod() {
        println("old")
    }

    fun caller() {
        oldMethod()
    }

    // Type reference to deprecated class
    fun useOldClass(): OldClass? = null
}
