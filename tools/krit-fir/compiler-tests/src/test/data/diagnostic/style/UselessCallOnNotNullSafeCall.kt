// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11, 13, 17, 24
// A null-tolerant call after `?.` only runs on a non-null receiver, so it is useless whatever the
// receiver's declared type is. detekt 1.23.8 reports these too.
package test

class Holder(val label: String?, val rows: List<String>?)

fun safeOnNonNull(x: String): String? = x?.<!UselessCallOnNotNull!>orEmpty<!>()

fun safeOnNullable(x: String?): String? = x?.<!UselessCallOnNotNull!>orEmpty<!>()

fun safeNullOrEmpty(x: String?): Boolean? = x?.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun safeNullOrBlank(x: CharSequence?): Boolean? = x?.<!UselessCallOnNotNull!>isNullOrBlank<!>()

fun safeList(x: List<String>?): List<String>? = x?.<!UselessCallOnNotNull!>orEmpty<!>()

fun safeProperty(h: Holder): String? = h.label?.<!UselessCallOnNotNull!>orEmpty<!>()

fun safePropertyList(h: Holder): Boolean? = h.rows?.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun safeMultiline(x: String?): String? = x
    ?.<!UselessCallOnNotNull!>orEmpty<!>()

// The safe call on the property, not the call: `x?.label` can be null.
fun safeChainResult(x: Holder?): String = x?.label.orEmpty()
