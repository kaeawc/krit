package fixtures.positive.potentialbugs

class ExplicitGarbageCollectionCall {
    fun cleanup() {
        System.gc()
    }
}
