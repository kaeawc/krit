package fixtures.negative.potentialbugs

class UnnecessarySafeCall {
    fun example(nullable: String?) {
        val len = nullable?.length
    }

    // Chained safe calls — upstream ?. makes receiver nullable
    fun chainedSafeCalls(content: Content?) {
        val seconds = content?.dataMessage?.expireTimer?.seconds
    }

    // Dotted member access — bare name resolution is unreliable
    fun dottedAccess(obj: Wrapper) {
        val value = obj.property?.name
    }

    // Nullable parameter with default — safe call is justified
    fun withNullableDefault(s: String? = null) {
        val len = s?.length
    }
}
