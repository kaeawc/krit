package fixtures.negative.potentialbugs

class CharArrayToStringCall {
    fun convert(chars: CharArray): String {
        return String(chars)
    }
}
