// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class Label(val label: String)

fun Label?.toString(): String = this?.label ?: ""

class Holder {
    var mutableProp: String? = null
}

class Negatives {
    fun safeCall(value: String?): String? = value?.toString()

    fun safeCallWithFallback(value: String?): String = value?.toString() ?: ""

    fun nonNullReceiver(value: String): String = value.toString()

    fun projectExtension(label: Label?): String = label.toString()

    fun stableSmartCast(value: String?): String {
        if (value != null) {
            return value.toString()
        }
        return ""
    }

    fun elvisFirst(value: String?): String = (value ?: "").toString()

    fun notNullAssertion(value: String?): String = value!!.toString()

    fun commentsAndStrings(): String {
        // value.toString()
        return "value.toString()"
    }

    fun nonNullTemplate(value: String): String = "value=$value"

    fun nonNullExpressionTemplate(value: String?): String = "length=${value?.length ?: 0}"

    fun assertedTemplate(value: String?): String = "value=${value!!}"

    fun smartCastTemplate(value: String?): String {
        if (value == null) return ""
        return "value=$value"
    }

    fun primitiveTemplate(count: Int, flag: Boolean): String = "count=$count flag=$flag"

    fun nullLiteralTemplate(): String = "value=${null}"

    fun platformReceiver(builder: java.lang.StringBuilder): String = builder.toString()

    fun toStringWithRadix(value: Int): String = value.toString(16)

    fun implicitReceiver(): String = toString()

    // A non-null implicit receiver calls the member `Any.toString()`.
    fun nonNullScopeReceiver(value: String): String = with(value) { toString() }
}

fun String.nonNullExtensionReceiver(): String = toString()
