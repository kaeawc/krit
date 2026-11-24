// go-lines: 25
package notnulloperator.negative

fun nullableAssertion(value: String?, values: Map<String, String>) {
    println(value!!)
    println(values["missing"]!!)
    val branch = if (values.isEmpty()) "value" else null
    println(branch!!)
    println((value as? CharSequence)!!)
}

fun <Value> nullableBoundAssertion(value: Value) {
    println(value!!)
}

fun platformAssertion() {
    val platform = System.getProperty("assertion.property")
    println(platform!!)
    println(System.getProperty("assertion.property")!!)
}

fun lambdaAssertion(value: String?) {
    value.apply { println(this!!) }
    // FIR deliberately skips lambda this even when resolution proves non-nullness; Go types apply's this.
    "value".apply { println(this!!) }
}
