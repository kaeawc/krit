// EXPECTED-KOTLINC-ERROR: CONFLICTING_OVERLOADS
// Two top-level functions with identical name and parameter types.
fun greet(name: String): String = "hi $name"

fun greet(name: String): String = "hello $name"
