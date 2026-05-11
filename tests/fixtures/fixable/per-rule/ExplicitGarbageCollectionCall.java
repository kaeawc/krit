package fixtures.positive.potentialbugs;

class ExplicitGarbageCollectionCall {
    void cleanup() {
        System.gc();
    }
}
