// go-lines: none
package unnecessarysafecall.negative

class NullableSafeCallHolder(val text: String?)

fun nullableSafeCallCases(value: String?, holder: NullableSafeCallHolder?, values: Map<String, String>) {
    println(value?.length)
    println(holder?.text?.length)
    println((value as? CharSequence)?.length)
    println(values["key"]?.length)
    with(value) { println(this?.length) }
    value.let { println(it?.length) }
}

fun <T> nullableSafeCallGeneric(value: T) = value?.hashCode()
fun <T : Any?> nullableSafeCallExplicitBound(value: T) = value?.hashCode()

fun platformSafeCall() {
    val property = System.getProperty("unnecessary.safe.call")
    println(property?.length)
}

class MutableNullableSafeCall(var text: String?) {
    fun read() {
        if (text != null) println(text?.length)
    }
}

typealias NullableSafeCallAlias = String?
fun nullableAliasSafeCall(value: NullableSafeCallAlias) = value?.length
