// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 14, 24, 29, 35, 40, 48, 60
// Values K2 cannot smart-cast (a member var, an open val, a val with a custom
// getter, a delegate) but that an is-check just proved to be a CharArray. Go
// reports them, and K2 keeps the unstable smart cast on the receiver.
package test

class VarHolder(var data: Any) {
    fun f() = if (data is CharArray) <!CharArrayToStringCall!>data.toString()<!> else ""

    fun afterCall(): String {
        if (data is CharArray) {
            println()
            return <!CharArrayToStringCall!>data.toString()<!>
        }
        return ""
    }

    // Go misses this: it narrows the bare name only, not `this.data`.
    fun explicitThis() = if (this.data is CharArray) <!CharArrayToStringCall!>this.data.toString()<!> else ""

    fun guard(): String {
        if (data !is CharArray) return ""
        return <!CharArrayToStringCall!>data.toString()<!>
    }
}

open class OpenHolder(open val data: Any) {
    fun f() = if (data is CharArray) <!CharArrayToStringCall!>data.toString()<!> else ""
}

class GetterHolder {
    val data: Any get() = CharArray(1)

    fun f() = if (data is CharArray) <!CharArrayToStringCall!>data.toString()<!> else ""
}

class MutableBox(var data: Any) {
    fun f() = when (data) {
        is CharArray -> <!CharArrayToStringCall!>data.toString()<!>
        else -> ""
    }
}

fun delegated(input: Any): String {
    val v: Any by lazy { input }
    if (v is CharArray) {
        return <!CharArrayToStringCall!>v.toString()<!>
    }
    return ""
}

// Go reports this because it keys the smart cast by the name `data`; the
// property is reassigned to a String before the call, so the value is not a
// CharArray.
class ReassignedHolder(var data: Any) {
    fun f(): String {
        if (data is CharArray) {
            data = "text"
            return data.toString()
        }
        return ""
    }
}
