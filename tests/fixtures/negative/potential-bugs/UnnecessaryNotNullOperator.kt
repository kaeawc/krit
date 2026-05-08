package fixtures.negative.potentialbugs

class UnnecessaryNotNullOperator {
    fun example(nullable: String?) {
        val x = nullable!!
    }

    // Dotted member access — bare name resolution is unreliable
    fun dottedAccess(harness: TestHarness) {
        val group = harness.group!!
    }
}
