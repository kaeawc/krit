package potentialbugs

class NullableToStringCall(val label: String)

fun NullableToStringCall?.toString(): String = this?.label ?: ""

fun formatSafeValues(value: String?, nonNull: String, user: NullableToStringCall?): String {
    // value.toString()
    val sample = "value.toString()"
    val safe = value?.toString()
    val fallback = value?.toString() ?: ""
    val directNonNull = nonNull.toString()
    val custom = user.toString()
    val unresolved = "missing=$missing"

    return sample + safe + fallback + directNonNull + custom + unresolved
}
