package comments

fun validSample(): String = "ok"

/**
 * References a sample that still exists.
 * @sample comments.validSample
 */
fun documentedApi(): String {
    return validSample()
}
