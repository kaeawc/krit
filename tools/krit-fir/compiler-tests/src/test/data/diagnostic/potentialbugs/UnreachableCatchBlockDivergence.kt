// RENDER_DIAGNOSTICS_FULL_TEXT
// Where FIR's resolved exception hierarchy and the Go rule disagree. Go
// compares the catch types as written: equal text is a duplicate, otherwise it
// looks both simple names up in a fixed table of well-known exceptions. FIR
// compares the resolved classes, so it drops Go findings whose later clause is
// not really shadowed and adds the shadowed clauses Go's table cannot see.
package test

import java.io.IOException
import java.io.IOException as IoErr
import java.net.SocketException
import java.net.SocketTimeoutException

typealias StorageFailure = IOException

fun risky() {
    throw IOException("fail")
}

// Go reports this because its table lists SocketException as a supertype of
// SocketTimeoutException. FIR is correct to drop it: SocketTimeoutException
// extends InterruptedIOException, not SocketException, so the clause is
// reachable.
fun socketTimeoutIsNotASocketException() {
    try {
        risky()
    } catch (e: SocketException) {
        println(e)
    } catch (e: SocketTimeoutException) {
        println(e)
    }
}

// A project class that reuses a well-known exception name.
class FileNotFoundException : Exception()

// Go reports this because its table says FileNotFoundException extends
// IOException. FIR is correct to drop it: this FileNotFoundException is the
// project class above, which extends Exception, so IOException does not catch
// it.
fun projectLookalike() {
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } catch (e: FileNotFoundException) {
        println(e)
    }
}

// Go misses this: its table only knows simple names, and the qualified
// java.io.IOException is not one. FIR resolves the qualified names.
fun qualifiedNames() {
    try {
        risky()
    } catch (e: java.lang.Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: java.io.IOException) {
        println(e)
    }
}

// Go misses this duplicate: the texts differ, but kotlin.Exception is an alias
// of java.lang.Exception, so both clauses catch the same class.
fun aliasDuplicate() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: java.lang.Exception) {
        println(e)
    }
}

// Go misses this duplicate: the texts differ, but kotlin.Exception and
// java.lang.Exception are the same class however the name is written.
fun kotlinQualifiedDuplicate() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: kotlin.Exception) {
        println(e)
    }
}

// Go misses this duplicate: the texts differ and the import alias IoErr is not
// in its table. The alias names java.io.IOException, so both clauses catch the
// same class.
fun importAlias() {
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IoErr) {
        println(e)
    }
}

// Go misses this: the unreachable clause names its type through the import
// alias, which Go's table does not know.
fun importAliasSubtype() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IoErr) {
        println(e)
    }
}

// Go misses this: the project type alias is not in its table.
fun projectTypeAlias() {
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: StorageFailure) {
        println(e)
    }
}

// Go misses these: project exceptions are not in its table.
open class RepositoryFailure : RuntimeException()

class MissingRow : RepositoryFailure()

fun projectHierarchy() {
    try {
        risky()
    } catch (e: RuntimeException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: RepositoryFailure) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: MissingRow) {
        println(e)
    }
}

// Go misses these: nested project exceptions are not in its table, and it
// compares the written names, so Holder.Failure and Holder.Companion.Inner are
// unknown to it.
class Holder {
    open class Failure : IOException()

    companion object {
        class Inner : Failure()
    }
}

fun nestedClasses() {
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: Holder.Failure) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: Holder.Companion.Inner) {
        println(e)
    }
}

// Go misses this: a local exception class is not in its table.
fun localClass() {
    class LocalFailure : IOException()
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: LocalFailure) {
        println(e)
    }
}

// Go skips a parenthesized catch type (it only reads a bare user type), so it
// misses this; FIR sees through the parentheses.
fun parenthesizedType() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: (IOException)) {
        println(e)
    }
}

// Go misses this: with the `catch` keyword on its own line, its tree-sitter
// parse does not attach the clause to the try. Kotlin does, and the clause is
// shadowed.
fun catchOnItsOwnLine() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    }
    <!UnreachableCatchBlock!>catch<!> (e: IOException) {
        println(e)
    }
}
