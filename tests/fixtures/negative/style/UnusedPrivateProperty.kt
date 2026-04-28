package fixtures.negative.style

private val direct = 1

fun foo() = direct

class BadgeSpriteTransformation {
    private val id = "BadgeSpriteTransformation.$VERSION"

    fun key(): String = id

    companion object {
        private const val VERSION = 3
    }
}

object Fonts {
    private const val BASE_STATIC_BUCKET_URI = "https://cdn.example.test/story-fonts"
    private const val MANIFEST = "manifest.json"

    fun manifestPath(version: String): String = "$BASE_STATIC_BUCKET_URI/$version/$MANIFEST"
}

class GroupsV2StateProcessor(private val groupId: String) {
    private val logPrefix = "[$groupId]"

    fun message(): String = "$logPrefix Local state and server state are equal"
}
