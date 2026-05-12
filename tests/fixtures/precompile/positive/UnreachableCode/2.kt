// EXPECTED-KOTLINC-ERROR: UNREACHABLE_CODE
fun afterThrow(): Int {
    throw IllegalStateException("boom")
    return 0
}
