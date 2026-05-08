package fixtures.negative.potentialbugs

class ExplicitGarbageCollectionCall {
    fun cleanup() {
        println("no gc")
    }
}
