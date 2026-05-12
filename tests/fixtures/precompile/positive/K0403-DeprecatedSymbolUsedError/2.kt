// EXPECTED-KOTLINC-ERROR: DEPRECATION_ERROR
@Deprecated(message = "Removed", level = DeprecationLevel.ERROR)
class OldType

fun makeOne(): OldType {
    return OldType()
}
