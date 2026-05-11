// EXPECTED-KOTLINC-ERROR: UNREACHABLE_CODE
fun loopBreakDead(xs: List<Int>): Int {
    for (x in xs) {
        if (x > 0) {
            break
            println("never")
        }
    }
    return 0
}
