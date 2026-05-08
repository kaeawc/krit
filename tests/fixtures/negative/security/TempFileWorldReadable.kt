package test

import java.io.File

fun ownerOnlyExplicit() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true, true)
}

fun ownerOnlyDefault() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true)
}

fun notFromCreateTempFile() {
    val t = File("/tmp/known.txt")
    t.setReadable(true, false)
}

fun localLookalikeReceiver() {
    // setReadable on a non-File receiver named the same as a temp file binding
    // in another function; intra-procedural binding lookup must not cross
    // function boundaries.
    val t = "not-a-file"
    t.toString()
}

fun bindingInDifferentFunctionDoesNotLeak() {
    val t = "string"
    t.length
}

fun makeReadableSeparate() {
    // No createTempFile binding visible in this function.
    val other = File("/tmp/known.txt")
    other.setReadable(true, false)
}
