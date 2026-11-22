// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package elvis.negative

typealias NullableElvisText = String?

class ElvisNegative {
    var unstable: String? = "value"
    val custom: String? get() = null

    fun nullable(value: String?, items: Map<String, String>, list: List<String?>, other: Any): String {
        val direct = value ?: "fallback"
        val safeCall = value?.toString() ?: "fallback"
        val safeCast = (other as? String) ?: "fallback"
        val indexed = items["missing"] ?: "fallback"
        val nullableElement = list[0] ?: "fallback"
        val getter = custom ?: "fallback"
        val aliased: NullableElvisText = value
        val aliasResult = aliased ?: "fallback"
        if (unstable != null) {
            val unproven = unstable ?: "fallback"
            return unproven
        }
        return direct + safeCall + safeCast + indexed + nullableElement + getter + aliasResult
    }
}

fun <T> elvisUnbounded(value: T, fallback: T): T = value ?: fallback

fun <T : Any?> elvisNullableBound(value: T, fallback: T): T = value ?: fallback

fun <T : U, U> elvisIndirectNullableBound(value: T, fallback: T): T = value ?: fallback

// A Java platform type is not evidence of non-null, even when its lower bound is String.
fun elvisPlatform(): String = System.getenv("KRIT_ELVIS_VALUE") ?: "fallback"

fun elvisPlatformLocal(): String {
    val platform = System.getProperty("krit.elvis.value")
    return platform ?: "fallback"
}

fun elvisPlatformMember(properties: java.util.Properties): String = properties.getProperty("value") ?: "fallback"

// Substitution of String into a Java generic still leaves a flexible return type, not proof.
fun elvisPlatformGeneric(callable: java.util.concurrent.Callable<String>): String = callable.call() ?: "fallback"

class ElvisConservativeSmartCast(val text: String?) {
    fun refined(): String {
        if (text == null) return "missing"
        // Pinned false negative: stable non-local property smart casts are ignored without reading declaration source.
        // This remains safe when a dependency is compiled separately and that smart cast is no longer legal.
        return text ?: "fallback"
    }
}
