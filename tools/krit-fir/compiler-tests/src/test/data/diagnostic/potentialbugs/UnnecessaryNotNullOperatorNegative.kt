// go-lines: none
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
    // Deliberate conservative miss: skip lambda this even when resolution proves non-nullness.
    "value".apply { println(this!!) }
}
