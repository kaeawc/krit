package comments

class ExistingType

fun existingHelper(): String = "ok"

/**
 * Resolves valid references to [ExistingType], [comments.existingHelper], [String],
 * and [kotlin.collections.List].
 *
 * Markdown [external](https://example.com) links are not KDoc symbols.
 */
fun loadValidLink(): ExistingType {
    return ExistingType()
}

/**
 * References an out-of-scope symbol intentionally.
 * [MissingType]
 */
@Suppress("KdocLinkValidation")
fun suppressedDocLink(): String {
    return existingHelper()
}
