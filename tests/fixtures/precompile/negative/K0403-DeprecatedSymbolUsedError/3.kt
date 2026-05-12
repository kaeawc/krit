// Negative: the annotation declaration site is not a use site, and
// imports are skipped.
import kotlin.DeprecationLevel

@Deprecated("Use newApi", level = DeprecationLevel.ERROR)
fun oldApi() {}

fun unrelated(): Int = 1
