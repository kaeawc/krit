package comments

fun validSample(): String = "ok"

/**
 * References a sample that still exists.
 * @sample comments.validSample
 */
fun documentedApi(): String {
    return validSample()
}

/**
 * References a sample supplied outside this analysis scope.
 * @sample comments.externalSample
 */
@Suppress("SampleAnnotationFreshness")
fun suppressedDocumentedApi(): String {
    return "external"
}
