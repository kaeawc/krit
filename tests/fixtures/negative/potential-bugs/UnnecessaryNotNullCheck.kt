package fixtures.negative.potentialbugs

class UnnecessaryNotNullCheck {
    private val observed = mutableListOf<String>()

    fun name(seed: String?): String? = seed?.trim()

    fun check(value: String?): Boolean {
        if (value != null) {
            observed += value
        }

        return name(value) == null
    }
}
