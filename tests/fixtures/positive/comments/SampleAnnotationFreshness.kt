package comments

fun existingSample(): String = "ok"

/**
 * References a sample that no longer exists.
 * @sample comments.removedSample
 */
fun documentedApi(): String {
    return existingSample()
}
