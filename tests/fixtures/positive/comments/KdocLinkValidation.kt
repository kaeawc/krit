package comments

class ExistingType

/**
 * Resolves a broken reference to [MissingType] and [comments.removed.helper].
 */
fun loadBrokenLink(): ExistingType {
    return ExistingType()
}
