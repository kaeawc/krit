// go-lines: 47, 51, 60, 65, 70
package unnecessarysafecall.resolution

class ResolvedSafeCallHolder(val text: String)

// Go skips dotted receivers; the resolved property signature proves String is non-null.
fun resolvedSafeCallProperty(holder: ResolvedSafeCallHolder) = holder.text<!UnnecessarySafeCall!>?.<!>length

// Go skips indexing; Array<String>.get returns non-null String, unlike Map.get.
fun resolvedSafeCallIndex(values: Array<String>) = values[0]<!UnnecessarySafeCall!>?.<!>length

// Go rejects generic parameters; the Any upper bound proves this T cannot be null.
fun <T : Any> resolvedSafeCallBound(value: T) = value<!UnnecessarySafeCall!>?.<!>hashCode()

// Go skips mutable declarations; a declared String var is nevertheless non-null.
fun resolvedSafeCallMutable() {
    var text: String = "first"
    text = "second"
    println(text<!UnnecessarySafeCall!>?.<!>length)
}

// Go skips an uncertain call initializer; resolution proves the lambda returns String.
fun resolvedSafeCallLambda() {
    val produce = { "value" }
    val text = produce()
    println(text<!UnnecessarySafeCall!>?.<!>length)
}

// FIR follows a stable same-module smart cast; Go skips the nullable declaration.
fun resolvedSafeCallSmartCast(value: String?) {
    if (value != null) println(value<!UnnecessarySafeCall!>?.<!>length)
}

// Go skips dotted receivers; the getter's declared return type proves non-null without reading its body.
class ResolvedSafeCallGetter {
    val text: String get() = "text"
}
fun resolvedSafeCallGetter(holder: ResolvedSafeCallGetter) = holder.text<!UnnecessarySafeCall!>?.<!>length

// Go skips dotted receivers; an anonymous object's property signature also proves non-null.
fun resolvedSafeCallAnonymous() {
    val holder = object { val text: String = "text" }
    println(holder.text<!UnnecessarySafeCall!>?.<!>length)
}

// FIR and Go both type extension this as its receiver; the non-null String receiver proves the report.
fun String.resolvedSafeCallThis() = this<!UnnecessarySafeCall!>?.<!>length

// FIR and Go both recognize the stable same-module cast; the type test proves String is non-null here.
fun resolvedSafeCallNonNullSmartCast(value: Any) {
    if (value is String) println(value<!UnnecessarySafeCall!>?.<!>length)
}

class LambdaReceiverSettings {
    fun reset() {}
}

// FIR and Go resolve this to the non-null apply lambda receiver.
fun lambdaReceiverApply(settings: LambdaReceiverSettings) {
    settings.apply { this<!UnnecessarySafeCall!>?.<!>reset() }
}

// FIR resolves this to the non-null with lambda receiver.
fun lambdaReceiverWith(settings: LambdaReceiverSettings) {
    with(settings) { this<!UnnecessarySafeCall!>?.<!>reset() }
}

// FIR resolves this to the non-null run lambda receiver.
fun lambdaReceiverRun(settings: LambdaReceiverSettings) {
    settings.run { this<!UnnecessarySafeCall!>?.<!>reset() }
}
