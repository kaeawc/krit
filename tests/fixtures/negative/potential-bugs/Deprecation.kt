package fixtures.negative.potentialbugs

class ActiveClass

class Deprecation {
    fun newMethod() {
        println("new")
    }

    fun caller() {
        newMethod()
    }

    // Type reference to non-deprecated class
    fun useActiveClass(): ActiveClass? = null
}

// Annotations that merely contain the word "Deprecated" are not deprecations:
// an opt-in marker whose name mentions deprecation, and a project's own marker.
@RequiresOptIn
annotation class DeprecatedForRemovalCompilerApi

annotation class DeprecatedMarker

class OptInCaller {
    @OptIn(DeprecatedForRemovalCompilerApi::class)
    private fun setReceiverParameter() {}

    @DeprecatedMarker
    fun marked() {}

    fun caller() {
        setReceiverParameter()
        marked()
    }
}
