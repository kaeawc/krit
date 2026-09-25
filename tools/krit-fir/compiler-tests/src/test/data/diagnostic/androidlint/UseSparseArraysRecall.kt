// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (recall): each call below constructs a java.util.HashMap with an
// Int or Long key, and FIR reports it. Go misses all of them: it reads only the
// type arguments written on the call, by the last identifier of a plain user
// type, and needs the call name HashMap.
package test

import java.util.HashMap as JavaHashMap

typealias Id = Int
typealias Flag = Boolean

// The key is named through a typealias (Go sees `Id`).
fun aliasedKey(): Map<Id, String> = <!UseSparseArrays!>HashMap<Id, String>()<!>

// A parenthesized key type is not a plain user type to Go.
fun parenthesizedKey(): Map<Int, String> = <!UseSparseArrays!>HashMap<(Int), String>()<!>

// An import alias of java.util.HashMap (Go sees the call name `JavaHashMap`).
fun importAlias(): Map<Int, String> = <!UseSparseArrays!>JavaHashMap<Int, String>()<!>

// Type arguments inferred from the expected type (Go sees no type arguments).
fun inferred(): HashMap<Long, String> = <!UseSparseArrays!>HashMap()<!>

fun inferredLocal() {
    val map: MutableMap<Int, Boolean> = <!UseSparseArrays!>HashMap()<!>
    map[1] = true
}

// Type arguments inferred from a constructor argument.
fun inferredFromArgument(source: Map<Int, String>): Map<Int, String> = <!UseSparseArrays!>HashMap(source)<!>

// Go reports this one too, but suggests SparseArray because it reads the value
// type `Flag` by name; FIR expands it to Boolean and suggests
// SparseBooleanArray. Same line, so the merge confirms Go's finding and keeps
// Go's message text.
fun aliasedValue(): Map<Int, Flag> = <!UseSparseArrays!>HashMap<Int, Flag>()<!>
