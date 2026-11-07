package fixtures.negative.potentialbugs

enum class Code(val code: Int) { First(1) }

class Avatar

class State {
    val currentAvatar: Avatar = Avatar()
}

class HasPlatformType {
    private val items = listOf(1, 2)

    private fun getState() = State()

    fun getName(): String = java.io.File("a").name

    // Kotlin expressions are not platform types
    fun fromCode(code: Int) = Code.entries.first { it.code == code }
    fun getCurrentAvatar() = getState().currentAvatar
    fun count() = items.size
}
