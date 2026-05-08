package style

fun example() {
    val x = 1_000_000
    val small = 1000       // 4 digits — below acceptableLength, should not flag
    val hundred = 100      // 3 digits — should not flag
    val tenThousand = 9999 // 4 digits — should not flag
}
