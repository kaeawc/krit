package fixtures.negative.potentialbugs

class CastToNullableType {
    fun cast(x: Any): String? {
        return x as? String
    }
}
