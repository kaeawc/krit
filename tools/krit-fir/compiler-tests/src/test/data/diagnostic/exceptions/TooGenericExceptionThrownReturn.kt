// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 14, 19, 23
// A return whose value throws a generic exception: both report the throw once,
// on the line of the throw.
package test

fun lookup(m: Map<String, String>, key: String): String {
    return m[key] ?: <!TooGenericExceptionThrown!>throw Exception("missing $key")<!>
}

// Split over two lines: the throw line is reported, not the return line.
fun lookupSplit(m: Map<String, String>, key: String): String {
    return m[key]
        ?: <!TooGenericExceptionThrown!>throw RuntimeException("missing $key")<!>
}

// The same through an `if` branch.
fun checked(ok: Boolean, value: String): String {
    return if (ok) value else <!TooGenericExceptionThrown!>throw Error("bad")<!>
}

// An expression body has no return: both report the throw once.
fun lookupBody(m: Map<String, String>, key: String): String = m[key] ?: <!TooGenericExceptionThrown!>throw Exception("missing")<!>
