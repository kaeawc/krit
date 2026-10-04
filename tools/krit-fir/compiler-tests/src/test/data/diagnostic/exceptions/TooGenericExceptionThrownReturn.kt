// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12x2, 18, 19, 24x2, 28
// A return whose value throws a generic exception. Go dispatches on every jump
// expression, and the first call inside the return is the constructor of the
// throw it contains, so Go reports the one throw twice: once for the return
// and once for the throw.
package test

// Divergence (duplicate): Go reports this line twice. One exception is thrown,
// so it is reported once, on the throw.
fun lookup(m: Map<String, String>, key: String): String {
    return m[key] ?: <!TooGenericExceptionThrown!>throw<!> Exception("missing $key")
}

// Divergence (duplicate): split over two lines, Go reports the return line as
// well as the throw line. The throw is reported once, on its own line.
fun lookupSplit(m: Map<String, String>, key: String): String {
    return m[key]
        ?: <!TooGenericExceptionThrown!>throw<!> RuntimeException("missing $key")
}

// Divergence (duplicate): the same through an `if` branch.
fun checked(ok: Boolean, value: String): String {
    return if (ok) value else <!TooGenericExceptionThrown!>throw<!> Error("bad")
}

// An expression body has no return: both report the throw once.
fun lookupBody(m: Map<String, String>, key: String): String = m[key] ?: <!TooGenericExceptionThrown!>throw<!> Exception("missing")
