package potentialbugs

class NullableToStringCall {
    fun findName(useFallback: Boolean): String? = if (useFallback) "fallback" else null
}

fun formatNullableValues(value: String?, repo: NullableToStringCall): String {
    val direct = value.toString()
    val complex = repo.findName(false).toString()
    val multiline = value
        .toString()
    val templated = "value=$value"

    return direct + complex + multiline + templated
}
