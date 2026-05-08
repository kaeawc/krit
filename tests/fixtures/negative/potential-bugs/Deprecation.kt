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
