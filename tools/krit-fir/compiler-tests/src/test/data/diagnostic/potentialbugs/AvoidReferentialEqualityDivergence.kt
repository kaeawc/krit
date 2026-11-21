// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 49, 74, 78, 81, 83, 95, 99, 112, 113, 116, 117, 126, 127, 130
package test

typealias Text = String
// Both Go and FIR resolve this simple same-file alias to String.
fun alias(a: Text, b: Text) = <!AvoidReferentialEquality!>a === b<!>

// Go exempts indexed operands even when the element type is String.
fun indexed(texts: List<String>, candidate: String) = <!AvoidReferentialEquality!>texts[0] === candidate<!>

// Go exempts iteration lambda parameters by syntax; FIR sees a String.
fun iteration(texts: List<String>, candidate: String) = texts.any { <!AvoidReferentialEquality!>it === candidate<!> }

// Go treats uppercase names as sentinel references; FIR sees a String.
fun uppercase(TEXT: String, candidate: String) = <!AvoidReferentialEquality!>TEXT === candidate<!>

// Go's view-name heuristic suppresses even a String operand named view.
fun viewName(view: String, candidate: String) = <!AvoidReferentialEquality!>view === candidate<!>

// Go's enum-reference and sentinel-alias heuristics also exempt String fields.
object Constants { val TEXT: String = "a" }
fun qualifiedConstant(candidate: String) = <!AvoidReferentialEquality!>candidate === Constants.TEXT<!>
fun sentinelAlias(candidate: String): Boolean {
    val alias = Constants.TEXT
    return <!AvoidReferentialEquality!>candidate !== alias<!>
}

// Both engines treat equal argument text as a same-instance fast path.
fun normalize(text: String) = text.trim()
fun repeatedLiteral() = normalize("a") === "a"

// Go treats this field access as an identity guard regardless of its type.
class Holder(val text: String) {
    fun same(candidate: String) = <!AvoidReferentialEquality!>this.text === candidate<!>
}

// Go treats a reference named original as a same-instance shortcut.
fun originalName(text: String, original: String) = <!AvoidReferentialEquality!>text === original<!>

// Go suppresses resource cleanup by text, including text in a literal.
fun cleanupText(a: String, b: String) {
    if (<!AvoidReferentialEquality!>a !== b<!>) println(".close()")
}

// A non-leading comparison is not a comparator identity fast path.
fun compare(a: String, b: String): Int {
    println("compare")
    if (<!AvoidReferentialEquality!>a === b<!>) return 0
    return a.compareTo(b)
}

// Both engines preserve safe-call and nested-argument same-instance probes.
class Normalizer { fun normalize(text: String): String = text.trim() }
fun safeCall(holder: Normalizer?, text: String) = holder?.normalize(text) === text
fun nestedArgument(text: String) = requireNotNull(normalize(text)) === text

// Both engines accept a zero return anywhere in the leading comparator if.
class TextComparator : Comparator<String> {
    override fun compare(a: String, b: String): Int {
        if (a === b) { println("compare"); return 0 }
        return a.compareTo(b)
    }
}

// FIR sees a Java platform String; Go cannot resolve this Java method's result.
fun platformString(candidate: Any) = <!AvoidReferentialEquality!>System.getProperty("text") === candidate<!>

// Both engines conservatively skip this type parameter despite its String bound.
fun <T : String> boundedType(text: T, candidate: Any) = text === candidate
// Widening to a concrete String supplies the resolved type in both engines.
fun <T : String> concreteBound(text: T, candidate: Any): Boolean {
    val concrete: String = text
    return <!AvoidReferentialEquality!>concrete === candidate<!>
}

// Known Go false positive: FIR respects the type parameter; Go's resolver expands the alias.
fun <Text> aliasShadow(a: Text, b: Any) = a === b

// The operand must itself be a call: neither Elvis nor property access is exempt.
fun elvisOperand(text: String) = <!AvoidReferentialEquality!>(normalize(text) ?: "") === text<!>
class Box(val value: String)
fun propertyOperand(text: String) = <!AvoidReferentialEquality!>Box(text).value === text<!>

// A zero return in the else branch is also a comparator fast path in both engines.
class ElseComparator : Comparator<String> {
    override fun compare(a: String, b: String): Int {
        if (a === b) return 1 else return 0
    }
}

// A sibling or nested helper's vararg does not shadow the nearest plain parameter.
class Matcher {
    fun anyOf(vararg names: String) = names.size
    fun same(names: String, candidate: Any) = <!AvoidReferentialEquality!>names === candidate<!>
}
fun nestedHelper(text: String, candidate: Any): Boolean {
    fun helper(vararg text: String) = text.size
    return <!AvoidReferentialEquality!>text === candidate<!>
}

// Both engines include receiver-chain arguments in same-instance probes.
fun receiverChain(text: String) = normalize(text).trim() === text
fun builderChain(text: String) = StringBuilder().append(text).toString() === text

// Both engines walk the whole safe call, including arguments in its receiver.
fun normalizeOrNull(text: String): String? = text.trim()
fun nullableReceiverChain(text: String) = normalizeOrNull(text)?.trim() === text
fun safeReceiverChain(holder: Normalizer?, text: String) = holder?.normalize(text)?.trim() === text

// FIR includes operator and named-argument calls; Go requires direct argument text.
fun suffix(a: String, text: String) = a + text === text
fun named(text: String) = normalize(text = text) === text

// FIR unwraps these call operands; Go requires a direct call expression.
fun parenthesizedCall(text: String) = (normalize(text)) === text
fun notNullCall(text: String) = normalize(text)!! === text

// Go deliberately limits the vararg correction to immediately enclosing functions.
class Tags {
    val tags: List<String> = emptyList()
    constructor(vararg tags: String, other: Any) { tags === other }
}

// Known Go false positives: the resolver uses a vararg's element type outside the narrow correction.
class PropertyTags(vararg val tags: String) { fun same(candidate: Any) = tags === candidate }
fun outerVararg(vararg texts: String, candidate: Any) { fun inner() = texts === candidate }
fun siblingShadow(vararg tags: String, candidate: Any): Boolean {
    run { val tags = 1; println(tags) }
    return tags === candidate
}
