// fir-parity: skip references undeclared TestHarness and ExternalThing; Go owns the unresolved fixture and compiling FIR negatives are covered by compiler-tests data
package fixtures.negative.potentialbugs

class UnnecessaryNotNullOperator {
    fun example(nullable: String?) {
        val x = nullable!!
    }

    // Dotted member access — bare name resolution is unreliable
    fun dottedAccess(harness: TestHarness) {
        val group = harness.group!!
    }

    fun unresolvedType(value: ExternalThing) {
        println(value!!)
    }
}
